package rpcv8

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
)

type SimulationFlag int

const (
	SkipValidateFlag SimulationFlag = iota + 1
	SkipFeeChargeFlag
)

const ExecutionStepsHeader string = "X-Cairo-Steps"

func (s *SimulationFlag) UnmarshalJSON(bytes []byte) (err error) {
	switch flag := string(bytes); flag {
	case `"SKIP_VALIDATE"`:
		*s = SkipValidateFlag
	case `"SKIP_FEE_CHARGE"`:
		*s = SkipFeeChargeFlag
	default:
		err = fmt.Errorf("unknown simulation flag %q", flag)
	}

	return
}

type SimulatedTransaction struct {
	TransactionTrace *vm.TransactionTrace `json:"transaction_trace,omitempty"`
	FeeEstimation    FeeEstimate          `json:"fee_estimation,omitempty"`
}

type TracedBlockTransaction struct {
	TraceRoot       *vm.TransactionTrace `json:"trace_root,omitempty"`
	TransactionHash *felt.Felt           `json:"transaction_hash,omitempty"`
}

/****************************************************
		Simulate Handlers
*****************************************************/

func (h *Handler) SimulateTransactions(id BlockID, transactions []BroadcastedTransaction,
	simulationFlags []SimulationFlag,
) ([]SimulatedTransaction, http.Header, *jsonrpc.Error) {
	return h.simulateTransactions(id, transactions, simulationFlags, false)
}

func (h *Handler) simulateTransactions(id BlockID, transactions []BroadcastedTransaction,
	simulationFlags []SimulationFlag, errOnRevert bool,
) ([]SimulatedTransaction, http.Header, *jsonrpc.Error) {
	skipFeeCharge := slices.Contains(simulationFlags, SkipFeeChargeFlag)
	skipValidate := slices.Contains(simulationFlags, SkipValidateFlag)

	httpHeader := http.Header{}
	httpHeader.Set(ExecutionStepsHeader, "0")

	state, closer, rpcErr := h.stateByBlockID(&id)
	if rpcErr != nil {
		return nil, httpHeader, rpcErr
	}
	defer h.callAndLogErr(closer, "Failed to close state in starknet_estimateFee")

	header, rpcErr := h.blockHeaderByID(&id)
	if rpcErr != nil {
		return nil, httpHeader, rpcErr
	}

	network := h.bcReader.Network()
	txns, classes, paidFeesOnL1, rpcErr := prepareTransactions(transactions, network)
	if rpcErr != nil {
		return nil, httpHeader, rpcErr
	}

	blockHashToBeRevealed, err := h.getRevealedBlockHash(header.Number)
	if err != nil {
		return nil, httpHeader, rpccore.ErrInternal.CloneWithData(err)
	}
	blockInfo := vm.BlockInfo{
		Header:                header,
		BlockHashToBeRevealed: blockHashToBeRevealed,
	}

	executionResults, err := h.vm.Execute(txns, classes, paidFeesOnL1, &blockInfo,
		state, network, skipFeeCharge, skipValidate, errOnRevert, true)
	if err != nil {
		return nil, httpHeader, handleExecutionError(err)
	}

	httpHeader.Set(ExecutionStepsHeader, strconv.FormatUint(executionResults.NumSteps, 10))

	simulatedTransactions, err := createSimulatedTransactions(executionResults, txns, header)
	if err != nil {
		return nil, httpHeader, rpccore.ErrInternal.CloneWithData(err)
	}

	return simulatedTransactions, httpHeader, nil
}

func prepareTransactions(transactions []BroadcastedTransaction, network *utils.Network) (
	[]core.Transaction, []core.Class, []*felt.Felt, *jsonrpc.Error,
) {
	txns := make([]core.Transaction, 0, len(transactions))
	var classes []core.Class
	paidFeesOnL1 := make([]*felt.Felt, 0)

	for idx := range transactions {
		txn, declaredClass, paidFeeOnL1, aErr := adaptBroadcastedTransaction(&transactions[idx], network)
		if aErr != nil {
			return nil, nil, nil, jsonrpc.Err(jsonrpc.InvalidParams, aErr.Error())
		}

		if paidFeeOnL1 != nil {
			paidFeesOnL1 = append(paidFeesOnL1, paidFeeOnL1)
		}

		txns = append(txns, txn)
		if declaredClass != nil {
			classes = append(classes, declaredClass)
		}
	}

	return txns, classes, paidFeesOnL1, nil
}

func handleExecutionError(err error) *jsonrpc.Error {
	if errors.Is(err, utils.ErrResourceBusy) {
		return rpccore.ErrInternal.CloneWithData(rpccore.ThrottledVMErr)
	}
	var txnExecutionError vm.TransactionExecutionError
	if errors.As(err, &txnExecutionError) {
		return makeTransactionExecutionError(&txnExecutionError)
	}
	return rpccore.ErrUnexpectedError.CloneWithData(err.Error())
}

func createSimulatedTransactions(
	executionResults vm.ExecutionResults, txns []core.Transaction, header *core.Header, //nolint: gocritic
) ([]SimulatedTransaction, error) {
	overallFees := executionResults.OverallFees
	traces := executionResults.Traces
	gasConsumed := executionResults.GasConsumed
	daGas := executionResults.DataAvailability
	if len(overallFees) != len(traces) || len(overallFees) != len(gasConsumed) ||
		len(overallFees) != len(daGas) || len(overallFees) != len(txns) {
		return nil, fmt.Errorf("inconsistent lengths: %d overall fees, %d traces, %d gas consumed, %d data availability, %d txns",
			len(overallFees), len(traces), len(gasConsumed), len(daGas), len(txns))
	}

	l1GasPriceWei := header.L1GasPriceETH
	l1GasPriceStrk := header.L1GasPriceSTRK
	l2GasPriceWei := &felt.Zero
	l2GasPriceStrk := &felt.Zero
	l1DataGasPriceWei := &felt.Zero
	l1DataGasPriceStrk := &felt.Zero

	if gasPrice := header.L2GasPrice; gasPrice != nil {
		l2GasPriceWei = gasPrice.PriceInWei
		l2GasPriceStrk = gasPrice.PriceInFri
	}
	if gasPrice := header.L1DataGasPrice; gasPrice != nil {
		l1DataGasPriceWei = gasPrice.PriceInWei
		l1DataGasPriceStrk = gasPrice.PriceInFri
	}

	simulatedTransactions := make([]SimulatedTransaction, len(overallFees))
	for i, overallFee := range overallFees {
		trace := traces[i]
		traces[i].ExecutionResources = &vm.ExecutionResources{
			L1Gas:                gasConsumed[i].L1Gas,
			L1DataGas:            gasConsumed[i].L1DataGas,
			L2Gas:                gasConsumed[i].L2Gas,
			ComputationResources: trace.TotalComputationResources(),
			DataAvailability: &vm.DataAvailability{
				L1Gas:     daGas[i].L1Gas,
				L1DataGas: daGas[i].L1DataGas,
			},
		}

		var l1GasPrice, l2GasPrice, l1DataGasPrice *felt.Felt
		feeUnit := feeUnit(txns[i])
		switch feeUnit {
		case WEI:
			l1GasPrice = l1GasPriceWei
			l2GasPrice = l2GasPriceWei
			l1DataGasPrice = l1DataGasPriceWei
		case FRI:
			l1GasPrice = l1GasPriceStrk
			l2GasPrice = l2GasPriceStrk
			l1DataGasPrice = l1DataGasPriceStrk
		}

		simulatedTransactions[i] = SimulatedTransaction{
			TransactionTrace: &traces[i],
			FeeEstimation: FeeEstimate{
				L1GasConsumed:     new(felt.Felt).SetUint64(gasConsumed[i].L1Gas),
				L1GasPrice:        l1GasPrice,
				L2GasConsumed:     new(felt.Felt).SetUint64(gasConsumed[i].L2Gas),
				L2GasPrice:        l2GasPrice,
				L1DataGasConsumed: new(felt.Felt).SetUint64(gasConsumed[i].L1DataGas),
				L1DataGasPrice:    l1DataGasPrice,
				OverallFee:        overallFee,
				Unit:              &feeUnit,
			},
		}
	}
	return simulatedTransactions, nil
}

type TransactionExecutionErrorData struct {
	TransactionIndex uint64          `json:"transaction_index"`
	ExecutionError   json.RawMessage `json:"execution_error"`
}

func makeTransactionExecutionError(err *vm.TransactionExecutionError) *jsonrpc.Error {
	return rpccore.ErrTransactionExecutionError.CloneWithData(TransactionExecutionErrorData{
		TransactionIndex: err.Index,
		ExecutionError:   err.Cause,
	})
}
