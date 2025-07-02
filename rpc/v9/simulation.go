package rpcv9

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
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
)

const ExecutionStepsHeader string = "X-Cairo-Steps"

type SimulatedTransaction struct {
	TransactionTrace *TransactionTrace `json:"transaction_trace,omitempty"`
	FeeEstimation    FeeEstimate       `json:"fee_estimation,omitzero"`
}

type TracedBlockTransaction struct {
	TraceRoot       *TransactionTrace `json:"trace_root,omitempty"`
	TransactionHash *felt.Felt        `json:"transaction_hash,omitempty"`
}

/****************************************************
		Simulate Handlers
*****************************************************/

func (h *Handler) SimulateTransactions(id *BlockID, transactions []BroadcastedTransaction,
	simulationFlags []rpcv6.SimulationFlag,
) ([]SimulatedTransaction, http.Header, *jsonrpc.Error) {
	return h.simulateTransactions(id, transactions, simulationFlags, false)
}

func (h *Handler) simulateTransactions(id *BlockID, transactions []BroadcastedTransaction,
	simulationFlags []rpcv6.SimulationFlag, errOnRevert bool,
) ([]SimulatedTransaction, http.Header, *jsonrpc.Error) {
	skipFeeCharge := slices.Contains(simulationFlags, rpcv6.SkipFeeChargeFlag)
	skipValidate := slices.Contains(simulationFlags, rpcv6.SkipValidateFlag)

	httpHeader := http.Header{}
	httpHeader.Set(ExecutionStepsHeader, "0")

	state, closer, rpcErr := h.stateByBlockID(id)
	if rpcErr != nil {
		return nil, httpHeader, rpcErr
	}
	defer h.callAndLogErr(closer, "Failed to close state in starknet_estimateFee")

	header, rpcErr := h.blockHeaderByID(id)
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
		state, network, skipFeeCharge, skipValidate, errOnRevert, true, true)
	if err != nil {
		return nil, httpHeader, handleExecutionError(err)
	}

	httpHeader.Set(ExecutionStepsHeader, strconv.FormatUint(executionResults.NumSteps, 10))

	simulatedTransactions, err := createSimulatedTransactions(&executionResults, txns, header)
	if err != nil {
		return nil, httpHeader, rpccore.ErrInternal.CloneWithData(err)
	}

	return simulatedTransactions, httpHeader, nil
}

func isVersion3(version *felt.Felt) bool {
	return version != nil && version.Equal(&rpcv6.RPCVersion3Value)
}

func checkTxHasSenderAddress(tx *BroadcastedTransaction) bool {
	return (tx.Transaction.Type == TxnDeclare ||
		tx.Transaction.Type == TxnInvoke) &&
		isVersion3(tx.Transaction.Version) &&
		tx.Transaction.SenderAddress == nil
}

func checkTxHasResourceBounds(tx *BroadcastedTransaction) bool {
	return (tx.Transaction.Type == TxnInvoke ||
		tx.Transaction.Type == TxnDeployAccount ||
		tx.Transaction.Type == TxnDeclare) &&
		isVersion3(tx.Transaction.Version) &&
		tx.Transaction.ResourceBounds == nil
}

func prepareTransactions(transactions []BroadcastedTransaction, network *utils.Network) (
	[]core.Transaction, []core.Class, []*felt.Felt, *jsonrpc.Error,
) {
	txns := make([]core.Transaction, len(transactions))
	var classes []core.Class
	paidFeesOnL1 := make([]*felt.Felt, 0)

	for idx := range transactions {
		// Check for missing required fields in struct that can't be validated by
		// jsonschema due to validation happening after omit empty
		//
		// TODO: as its expected that this will happen in other cases as well,
		// it might be a good idea to implement a custom validator and unmarshal handler
		// to solve this problem in a more elegant way
		if checkTxHasSenderAddress(&transactions[idx]) {
			return nil, nil, nil, jsonrpc.Err(jsonrpc.InvalidParams, "sender_address is required for this transaction type")
		}

		if checkTxHasResourceBounds(&transactions[idx]) {
			return nil, nil, nil, jsonrpc.Err(jsonrpc.InvalidParams, "resource_bounds is required for this transaction type")
		}

		txn, declaredClass, paidFeeOnL1, aErr := AdaptBroadcastedTransaction(&transactions[idx], network)
		if aErr != nil {
			return nil, nil, nil, jsonrpc.Err(jsonrpc.InvalidParams, aErr.Error())
		}

		if paidFeeOnL1 != nil {
			paidFeesOnL1 = append(paidFeesOnL1, paidFeeOnL1)
		}

		txns[idx] = txn
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
	executionResults *vm.ExecutionResults, txns []core.Transaction, header *core.Header,
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
		// Adapt transaction trace to rpc v9 trace
		trace := utils.HeapPtr(AdaptVMTransactionTrace(&traces[i]))

		// Add root level execution resources
		trace.ExecutionResources = &ExecutionResources{
			InnerExecutionResources: InnerExecutionResources{
				L1Gas: gasConsumed[i].L1Gas,
				L2Gas: gasConsumed[i].L2Gas,
			},
			L1DataGas: gasConsumed[i].L1DataGas,
		}

		// Compute data for FeeEstimate
		var l1GasPrice, l2GasPrice, l1DataGasPrice *felt.Felt

		// estimateFee only allows FRI as unit
		// https://github.com/starkware-libs/starknet-specs/blob/0bf403bfafbfbe0eaa52103a9c7df545bec8f73b/api/starknet_api_openrpc.json#L3586
		feeUnit := FRI
		// estimateMessageFee only allow WEI as unit
		// https://github.com/starkware-libs/starknet-specs/blob/0bf403bfafbfbe0eaa52103a9c7df545bec8f73b/api/starknet_api_openrpc.json#L3605
		if traces[i].Type == vm.TxnL1Handler {
			feeUnit = WEI
		}

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

		// Append simulated transaction (trace + fee estimate)
		simulatedTransactions[i] = SimulatedTransaction{
			TransactionTrace: trace,
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
