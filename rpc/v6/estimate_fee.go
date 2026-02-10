package rpcv6

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	rpccore "github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/utils"
)

type FeeUnit byte

const (
	WEI FeeUnit = iota
	FRI
)

func (u FeeUnit) MarshalText() ([]byte, error) {
	switch u {
	case WEI:
		return []byte("WEI"), nil
	case FRI:
		return []byte("FRI"), nil
	default:
		return nil, fmt.Errorf("unknown FeeUnit %v", u)
	}
}

type FeeEstimate struct {
	GasConsumed *felt.Felt `json:"gas_consumed"`
	GasPrice    *felt.Felt `json:"gas_price"`
	OverallFee  *felt.Felt `json:"overall_fee"`
	Unit        *FeeUnit   `json:"unit,omitempty"`
}

/****************************************************
		Estimate Fee Handlers
*****************************************************/

func (h *Handler) EstimateFee(
	ctx context.Context,
	broadcastedTxns BroadcastedTransactionInputs,
	simulationFlags []SimulationFlag,
	id BlockID,
) ([]FeeEstimate, *jsonrpc.Error) {
	result, err := h.simulateTransactions(
		ctx, id,
		broadcastedTxns.Data,
		append(simulationFlags, SkipFeeChargeFlag),
		true,
		true,
	)
	if err != nil {
		return nil, err
	}

	return utils.Map(result, func(tx SimulatedTransaction) FeeEstimate {
		return tx.FeeEstimation
	}), nil
}

func (h *Handler) EstimateMessageFee(
	ctx context.Context, msg MsgFromL1, id BlockID, //nolint:gocritic // MsgFromL1 passed by value
) (*FeeEstimate, *jsonrpc.Error) {
	feeEstimate, rpcErr := h.estimateMessageFee(ctx, msg, id, h.EstimateFee)
	if rpcErr != nil {
		return nil, rpcErr
	}
	return feeEstimate, nil
}

type estimateFeeHandler func(
	ctx context.Context,
	broadcastedTxns BroadcastedTransactionInputs,
	simulationFlags []SimulationFlag,
	id BlockID,
) ([]FeeEstimate, *jsonrpc.Error)

func (h *Handler) estimateMessageFee(
	ctx context.Context, msg MsgFromL1, id BlockID, //nolint:gocritic // MsgFromL1 passed by value
	f estimateFeeHandler,
) (*FeeEstimate, *jsonrpc.Error) {
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
	estimates, rpcErr := f(
		ctx,
		BroadcastedTransactionInputs{Data: []BroadcastedTransaction{tx}},
		nil,
		id,
	)
	if rpcErr != nil {
		if rpcErr.Code == rpccore.ErrTransactionExecutionError.Code {
			data := rpcErr.Data.(TransactionExecutionErrorData)
			return nil, MakeContractError(data.ExecutionError)
		}
		return nil, rpcErr
	}
	return &estimates[0], nil
}

type ContractErrorData struct {
	RevertError json.RawMessage `json:"revert_error"`
}

func MakeContractError(err json.RawMessage) *jsonrpc.Error {
	return rpccore.ErrContractError.CloneWithData(ContractErrorData{
		RevertError: err,
	})
}
