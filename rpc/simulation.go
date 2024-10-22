package rpc

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
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

//nolint:funlen,gocyclo
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

	txns := make([]core.Transaction, 0, len(transactions))
	var classes []core.Class

	paidFeesOnL1 := make([]*felt.Felt, 0)
	for idx := range transactions {
		txn, declaredClass, paidFeeOnL1, aErr := adaptBroadcastedTransaction(&transactions[idx], h.bcReader.Network())
		if aErr != nil {
			return nil, httpHeader, jsonrpc.Err(jsonrpc.InvalidParams, aErr.Error())
		}

		if paidFeeOnL1 != nil {
			paidFeesOnL1 = append(paidFeesOnL1, paidFeeOnL1)
		}

		txns = append(txns, txn)
		if declaredClass != nil {
			classes = append(classes, declaredClass)
		}
	}

	blockHashToBeRevealed, err := h.getRevealedBlockHash(header.Number)
	if err != nil {
		return nil, httpHeader, ErrInternal.CloneWithData(err)
	}
	blockInfo := vm.BlockInfo{
		Header:                header,
		BlockHashToBeRevealed: blockHashToBeRevealed,
	}
	overallFees, daGas, traces, numSteps, err := h.vm.Execute(txns, classes, paidFeesOnL1, &blockInfo,
		state, h.bcReader.Network(), skipFeeCharge, skipValidate, errOnRevert)

	httpHeader.Set(ExecutionStepsHeader, strconv.FormatUint(numSteps, 10))

	if err != nil {
		if errors.Is(err, utils.ErrResourceBusy) {
			return nil, httpHeader, ErrInternal.CloneWithData(throttledVMErr)
		}
		var txnExecutionError vm.TransactionExecutionError
		if errors.As(err, &txnExecutionError) {
			return nil, httpHeader, makeTransactionExecutionError(&txnExecutionError)
		}
		return nil, httpHeader, ErrUnexpectedError.CloneWithData(err.Error())
	}

	result := make([]SimulatedTransaction, 0, len(overallFees))
	for i, overallFee := range overallFees {
		feeUnit := feeUnit(txns[i])

		gasPrice := header.GasPrice
		if feeUnit == FRI {
			if gasPrice = header.GasPriceSTRK; gasPrice == nil {
				gasPrice = &felt.Zero
			}
		}

		dataGasPrice := &felt.Zero
		if header.L1DataGasPrice != nil {
			switch feeUnit {
			case FRI:
				dataGasPrice = header.L1DataGasPrice.PriceInFri
			case WEI:
				dataGasPrice = header.L1DataGasPrice.PriceInWei
			}
		}

		var gasConsumed *felt.Felt
		daGasL1DataGas := new(felt.Felt).SetUint64(daGas[i].L1DataGas)
		dataGasFee := new(felt.Felt).Mul(daGasL1DataGas, dataGasPrice)
		gasConsumed = new(felt.Felt).Sub(overallFee, dataGasFee)
		gasConsumed = gasConsumed.Div(gasConsumed, gasPrice) // division by zero felt is zero felt

		estimate := FeeEstimate{
			GasConsumed:     gasConsumed,
			GasPrice:        gasPrice,
			DataGasConsumed: daGasL1DataGas,
			DataGasPrice:    dataGasPrice,
			OverallFee:      overallFee,
			Unit:            utils.Ptr(feeUnit),
		}

		trace := traces[i]
		executionResources := trace.TotalExecutionResources()
		executionResources.DataAvailability = &vm.DataAvailability{
			L1Gas:     daGas[i].L1Gas,
			L1DataGas: daGas[i].L1DataGas,
		}
		traces[i].ExecutionResources = executionResources

		result = append(result, SimulatedTransaction{
			TransactionTrace: &traces[i],
			FeeEstimation:    estimate,
		})
	}

	return result, httpHeader, nil
}

type TransactionExecutionErrorData struct {
	TransactionIndex uint64 `json:"transaction_index"`
	ExecutionError   string `json:"execution_error"`
}

func makeTransactionExecutionError(err *vm.TransactionExecutionError) *jsonrpc.Error {
	return ErrTransactionExecutionError.CloneWithData(TransactionExecutionErrorData{
		TransactionIndex: err.Index,
		ExecutionError:   err.Cause.Error(),
	})
}
