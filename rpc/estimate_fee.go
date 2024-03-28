package rpc

import (
	"errors"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/utils"
)

/****************************************************
		Estimate Fee Handlers
*****************************************************/

func (h *Handler) EstimateFee(broadcastedTxns []BroadcastedTransaction,
	simulationFlags []SimulationFlag, id BlockID,
) ([]FeeEstimate, *jsonrpc.Error) {
	result, err := h.simulateTransactions(id, broadcastedTxns, append(simulationFlags, SkipFeeChargeFlag), false, true)
	if err != nil {
		return nil, err
	}

	return utils.Map(result, func(tx SimulatedTransaction) FeeEstimate {
		return tx.FeeEstimation
	}), nil
}

func (h *Handler) EstimateFeeV0_6(broadcastedTxns []BroadcastedTransaction,
	simulationFlags []SimulationFlag, id BlockID,
) ([]FeeEstimate, *jsonrpc.Error) {
	result, err := h.simulateTransactions(id, broadcastedTxns, append(simulationFlags, SkipFeeChargeFlag), true, true)
	if err != nil {
		return nil, err
	}

	return utils.Map(result, func(tx SimulatedTransaction) FeeEstimate {
		return tx.FeeEstimation
	}), nil
}

func (h *Handler) EstimateMessageFee(msg MsgFromL1, id BlockID) (*FeeEstimate, *jsonrpc.Error) { //nolint:gocritic
	return h.estimateMessageFee(msg, id, h.EstimateFee)
}

func (h *Handler) EstimateMessageFeeV0_6(msg MsgFromL1, id BlockID) (*FeeEstimate, *jsonrpc.Error) { //nolint:gocritic
	feeEstimate, rpcErr := h.estimateMessageFee(msg, id, h.EstimateFeeV0_6)
	if rpcErr != nil {
		return nil, rpcErr
	}

	feeEstimate.v0_6Response = true
	feeEstimate.DataGasPrice = nil
	feeEstimate.DataGasConsumed = nil

	return feeEstimate, nil
}

type estimateFeeHandler func(broadcastedTxns []BroadcastedTransaction,
	simulationFlags []SimulationFlag, id BlockID,
) ([]FeeEstimate, *jsonrpc.Error)

func (h *Handler) estimateMessageFee(msg MsgFromL1, id BlockID, f estimateFeeHandler) (*FeeEstimate, *jsonrpc.Error) { //nolint:gocritic
	calldata := make([]*felt.Felt, 0, len(msg.Payload)+1)
	// The order of the calldata parameters matters. msg.From must be prepended.
	calldata = append(calldata, new(felt.Felt).SetBytes(msg.From.Bytes()))
	for payloadIdx := range msg.Payload {
		calldata = append(calldata, &msg.Payload[payloadIdx])
	}
	tx := BroadcastedTransaction{
		Transaction: Transaction{
			Type:               TxnL1Handler,
			ContractAddress:    &msg.To,
			EntryPointSelector: &msg.Selector,
			CallData:           &calldata,
			Version:            &felt.Zero, // Needed for transaction hash calculation.
			Nonce:              &felt.Zero, // Needed for transaction hash calculation.
		},
		// Needed to marshal to blockifier type.
		// Must be greater than zero to successfully execute transaction.
		PaidFeeOnL1: new(felt.Felt).SetUint64(1),
	}
	estimates, rpcErr := f([]BroadcastedTransaction{tx}, nil, id)
	if rpcErr != nil {
		if rpcErr.Code == ErrTransactionExecutionError.Code {
			data := rpcErr.Data.(TransactionExecutionErrorData)
			return nil, makeContractError(errors.New(data.ExecutionError))
		}
		return nil, rpcErr
	}
	return &estimates[0], nil
}

type ContractErrorData struct {
	RevertError string `json:"revert_error"`
}

func makeContractError(err error) *jsonrpc.Error {
	return ErrContractError.CloneWithData(ContractErrorData{
		RevertError: err.Error(),
	})
}
