package rpcv6

import (
	"encoding/json"
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
	TransactionTrace *TransactionTrace `json:"transaction_trace,omitempty"`
	FeeEstimation    FeeEstimate       `json:"fee_estimation,omitempty"`
}

type TracedBlockTransaction struct {
	TraceRoot       *TransactionTrace `json:"trace_root,omitempty"`
	TransactionHash *felt.Felt        `json:"transaction_hash,omitempty"`
}

/****************************************************
		Simulate Handlers
*****************************************************/

// pre 13.1
func (h *Handler) SimulateTransactions(id BlockID, broadcastedTxns []BroadcastedTransaction,
	simulationFlags []SimulationFlag,
) ([]SimulatedTransaction, *jsonrpc.Error) {
	return h.simulateTransactions(id, broadcastedTxns, simulationFlags, false)
}

func (h *Handler) simulateTransactions(id BlockID, transactions []BroadcastedTransaction,
	simulationFlags []SimulationFlag, errOnRevert bool,
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

	network := h.bcReader.Network()
	txns, classes, paidFeesOnL1, rpcErr := prepareTransactions(transactions, network)
	if rpcErr != nil {
		return nil, rpcErr
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
		state, network, skipFeeCharge, skipValidate, errOnRevert, false)
	if err != nil {
		return nil, handleExecutionError(err)
	}

	simulatedTransactions := createSimulatedTransactions(&executionResults, header, txns)

	return simulatedTransactions, nil
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

func createSimulatedTransactions(executionResults *vm.ExecutionResults, header *core.Header,
	txns []core.Transaction,
) []SimulatedTransaction {
	daGas := executionResults.DataAvailability
	result := make([]SimulatedTransaction, len(txns))

	// For every transaction, we append its trace + fee estimate
	for i, overallFee := range executionResults.OverallFees {
		// Compute fee estimate
		feeUnit := feeUnit(txns[i])
		gasPrice := header.L1GasPriceETH
		if feeUnit == FRI {
			if gasPrice = header.L1GasPriceSTRK; gasPrice == nil {
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

		daGasL1DataGas := new(felt.Felt).SetUint64(daGas[i].L1DataGas)

		dataGasFee := new(felt.Felt).Mul(daGasL1DataGas, dataGasPrice)
		gasConsumed := new(felt.Felt).Sub(overallFee, dataGasFee)

		gasConsumed.Div(gasConsumed, gasPrice) // division by zero felt is zero felt

		estimate := FeeEstimate{
			GasConsumed: gasConsumed,
			GasPrice:    gasPrice,
			OverallFee:  overallFee,
			Unit:        &feeUnit,
		}
		result[i] = SimulatedTransaction{
			TransactionTrace: utils.HeapPtr(AdaptVMTransactionTrace(&executionResults.Traces[i])),
			FeeEstimation:    estimate,
		}
	}
	return result
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
