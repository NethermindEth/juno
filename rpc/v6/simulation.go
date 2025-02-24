package rpcv6

import (
	"errors"
	"fmt"
	"slices"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	rpccore "github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
)

type SimulationFlag int

const (
	SkipValidateFlag SimulationFlag = iota + 1
	SkipFeeChargeFlag
)

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

// pre 13.1
func (h *Handler) SimulateTransactions(id BlockID, transactions []BroadcastedTransaction,
	simulationFlags []SimulationFlag,
) ([]SimulatedTransaction, *jsonrpc.Error) {
	return h.simulateTransactions(id, transactions, simulationFlags, true, false)
}

//nolint:funlen
func (h *Handler) simulateTransactions(id BlockID, transactions []BroadcastedTransaction,
	simulationFlags []SimulationFlag, v0_6Response, errOnRevert bool,
) ([]SimulatedTransaction, *jsonrpc.Error) {
	skipFeeCharge := slices.Contains(simulationFlags, SkipFeeChargeFlag)
	skipValidate := slices.Contains(simulationFlags, SkipValidateFlag)

	state, closer, rpcErr := h.stateByBlockID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}
	defer h.callAndLogErr(closer, "Failed to close state in starknet_estimateFee")

	header, rpcErr := h.blockHeaderByID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}

	txns := make([]core.Transaction, len(transactions))
	var classes []core.Class

	paidFeesOnL1 := make([]*felt.Felt, 0)
	for idx := range transactions {
		txn, declaredClass, paidFeeOnL1, aErr := adaptBroadcastedTransaction(&transactions[idx], h.bcReader.Network())
		if aErr != nil {
			return nil, jsonrpc.Err(jsonrpc.InvalidParams, aErr.Error())
		}

		if paidFeeOnL1 != nil {
			paidFeesOnL1 = append(paidFeesOnL1, paidFeeOnL1)
		}

		txns[idx] = txn
		if declaredClass != nil {
			classes = append(classes, declaredClass)
		}
	}

	blockHashToBeRevealed, err := h.getRevealedBlockHash(header.Number)
	if err != nil {
		return nil, rpccore.ErrInternal.CloneWithData(err)
	}
	blockInfo := vm.BlockInfo{
		Header:                header,
		BlockHashToBeRevealed: blockHashToBeRevealed,
	}
	executionResults, err := h.vm.Execute(txns, classes, paidFeesOnL1, &blockInfo,
		state, h.bcReader.Network(), skipFeeCharge, skipValidate, errOnRevert)
	if err != nil {
		if errors.Is(err, utils.ErrResourceBusy) {
			return nil, rpccore.ErrInternal.CloneWithData(rpccore.ThrottledVMErr)
		}
		var txnExecutionError vm.TransactionExecutionError
		if errors.As(err, &txnExecutionError) {
			return nil, makeTransactionExecutionError(&txnExecutionError)
		}
		return nil, rpccore.ErrUnexpectedError.CloneWithData(err.Error())
	}
	overallFees := executionResults.OverallFees
	dataGasConsumed := executionResults.DataAvailability
	traces := executionResults.Traces

	result := make([]SimulatedTransaction, len(transactions))
	for i, overallFee := range overallFees {
		feeUnit := feeUnit(txns[i])

		gasPrice := header.L1GasPriceETH
		if feeUnit == FRI {
			if gasPrice = header.L1GasPriceSTRK; gasPrice == nil {
				gasPrice = &felt.Zero
			}
		}

		gasConsumed := overallFee.Clone()
		gasConsumed = gasConsumed.Div(gasConsumed, gasPrice) // division by zero felt is zero felt

		estimate := FeeEstimate{
			GasConsumed: gasConsumed,
			GasPrice:    gasPrice,
			OverallFee:  overallFee,
			Unit:        utils.Ptr(feeUnit),
		}

		if !v0_6Response {
			trace := traces[i]
			da := vm.NewDataAvailability(gasConsumed,
				new(felt.Felt).SetUint64(dataGasConsumed[i].L1DataGas), header.L1DAMode)
			traces[i].ExecutionResources = &vm.ExecutionResources{
				ComputationResources: trace.TotalComputationResources(),
				DataAvailability:     &da,
			}
		}

		result[i] = SimulatedTransaction{
			TransactionTrace: &traces[i],
			FeeEstimation:    estimate,
		}
	}

	return result, nil
}

type TransactionExecutionErrorData struct {
	TransactionIndex uint64 `json:"transaction_index"`
	ExecutionError   string `json:"execution_error"`
}

func makeTransactionExecutionError(err *vm.TransactionExecutionError) *jsonrpc.Error {
	return rpccore.ErrTransactionExecutionError.CloneWithData(TransactionExecutionErrorData{
		TransactionIndex: err.Index,
		ExecutionError:   err.Cause.Error(),
	})
}
