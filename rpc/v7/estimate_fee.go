package rpcv7

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
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
	GasConsumed     *felt.Felt `json:"gas_consumed"`
	GasPrice        *felt.Felt `json:"gas_price"`
	DataGasConsumed *felt.Felt `json:"data_gas_consumed"`
	DataGasPrice    *felt.Felt `json:"data_gas_price"`
	OverallFee      *felt.Felt `json:"overall_fee"`
	Unit            *FeeUnit   `json:"unit,omitempty"`
}

/****************************************************
		Estimate Fee Handlers
*****************************************************/

func (h *Handler) EstimateFee(
	ctx context.Context,
	broadcastedTxns BroadcastedTransactionInputs,
	simulationFlags []rpcv6.SimulationFlag,
	id BlockID,
) ([]FeeEstimate, http.Header, *jsonrpc.Error) {
	result, httpHeader, err := h.simulateTransactions(
		ctx,
		id,
		broadcastedTxns.Data,
		append(simulationFlags, rpcv6.SkipFeeChargeFlag),
		true,
		true,
	)
	if err != nil {
		return nil, httpHeader, err
	}

	return utils.Map(result, func(tx SimulatedTransaction) FeeEstimate {
		return tx.FeeEstimation
	}), httpHeader, nil
}

//nolint:gocritic // MsgFromL1 passed by value
func (h *Handler) EstimateMessageFee(
	ctx context.Context, msg rpcv6.MsgFromL1, id BlockID,
) (*FeeEstimate, http.Header, *jsonrpc.Error) {
	return h.estimateMessageFee(ctx, msg, id, h.EstimateFee)
}

type estimateFeeHandler func(
	ctx context.Context,
	broadcastedTxns BroadcastedTransactionInputs,
	simulationFlags []rpcv6.SimulationFlag,
	id BlockID,
) ([]FeeEstimate, http.Header, *jsonrpc.Error)

//nolint:gocritic // MsgFromL1 passed by value
func (h *Handler) estimateMessageFee(
	ctx context.Context, msg rpcv6.MsgFromL1, id BlockID,
	f estimateFeeHandler,
) (*FeeEstimate, http.Header, *jsonrpc.Error) {
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
	estimates, httpHeader, rpcErr := f(
		ctx,
		BroadcastedTransactionInputs{Data: []BroadcastedTransaction{tx}},
		nil,
		id,
	)
	if rpcErr != nil {
		if rpcErr.Code == rpccore.ErrTransactionExecutionError.Code {
			data := rpcErr.Data.(TransactionExecutionErrorData)
			return nil, httpHeader, MakeContractError(data.ExecutionError)
		}
		return nil, httpHeader, rpcErr
	}
	return &estimates[0], httpHeader, nil
}

type ContractErrorData struct {
	RevertError json.RawMessage `json:"revert_error"`
}

func MakeContractError(err json.RawMessage) *jsonrpc.Error {
	return rpccore.ErrContractError.CloneWithData(ContractErrorData{
		RevertError: err,
	})
}
