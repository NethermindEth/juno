package rpcv10

import (
	"context"
	"net/http"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/ethereum/go-ethereum/common"
)

// MsgFromL1 represents a message sent from L1 to L2.
type MsgFromL1 struct {
	// The address of the L1 contract sending the message.
	From common.Address `json:"from_address" validate:"required"`
	// The address of the L2 contract receiving the message.
	To felt.Felt `json:"to_address" validate:"required"`
	// The payload of the message.
	Payload  []felt.Felt `json:"payload" validate:"required"`
	Selector felt.Felt   `json:"entry_point_selector" validate:"required"`
}

/*
***************************************************

	Estimate Fee Handlers

****************************************************
*/
func (h *Handler) EstimateFee(
	ctx context.Context,
	broadcastedTxns BroadcastedTransactionInputs,
	estimateFlags []EstimateFlag,
	id *BlockID,
) ([]FeeEstimate, http.Header, *jsonrpc.Error) {
	simulationFlags := make([]SimulationFlag, 0, len(estimateFlags)+1)
	for _, flag := range estimateFlags {
		simulationFlag, err := flag.ToSimulationFlag()
		if err != nil {
			return nil, nil, jsonrpc.Err(jsonrpc.InvalidParams, err.Error())
		}
		simulationFlags = append(simulationFlags, simulationFlag)
	}

	txnResults, httpHeader, err := h.simulateTransactions(
		ctx,
		id,
		broadcastedTxns.Data,
		nil,
		append(simulationFlags, SkipFeeChargeFlag),
		true,
		true,
	)
	if err != nil {
		return nil, httpHeader, err
	}

	simulatedTransactions := txnResults.SimulatedTransactions
	feeEstimates := make([]FeeEstimate, len(simulatedTransactions))
	for i := range feeEstimates {
		feeEstimates[i] = simulatedTransactions[i].FeeEstimation
	}

	return feeEstimates, httpHeader, nil
}

type MessageFeePayload struct {
	L1Handler   core.L1HandlerTransaction
	PaidFeeOnL1 felt.Felt
}

func (h *Handler) EstimateMessageFee(
	ctx context.Context, msg *MsgFromL1, id *BlockID,
) (FeeEstimate, http.Header, *jsonrpc.Error) {
	calldata := make([]*felt.Felt, len(msg.Payload)+1)
	// msg.From needs to be the first element
	calldata[0] = felt.NewFromBytes[felt.Felt](msg.From.Bytes())
	for i := range msg.Payload {
		calldata[i+1] = &msg.Payload[i]
	}

	state, closer, rpcErr := h.stateByBlockID(id)
	if rpcErr != nil {
		return FeeEstimate{}, nil, rpcErr
	}
	defer h.callAndLogErr(closer, "Failed to close state in starknet_estimateMessageFee")

	if _, err := state.ContractClassHash(&msg.To); err != nil {
		return FeeEstimate{}, nil, rpccore.ErrContractNotFound
	}

	payload := MessageFeePayload{
		L1Handler: core.L1HandlerTransaction{
			TransactionHash:    nil,
			ContractAddress:    &msg.To,
			EntryPointSelector: &msg.Selector,
			CallData:           calldata,
			// Needed for L1_HANDLER transaction hash calculation. Every transaction to be
			// simulated must contain a valid tx hash; since we are just estimating the fees,
			// we can set the remaining required fields to zero, as we don't care about the
			// final tx hash.
			Version: (*core.TransactionVersion)(&felt.Zero),
			Nonce:   &felt.Zero,
		},
		// Needed to marshal to blockifier type.
		// Must be greater than zero to successfully execute transaction.
		PaidFeeOnL1: felt.FromUint64[felt.Felt](1),
	}

	result, httpHeader, err := h.simulateTransactions(
		ctx,
		id,
		nil,
		&payload,
		[]SimulationFlag{SkipFeeChargeFlag},
		true,
		true,
	)
	if err != nil {
		if err.Code == rpccore.ErrTransactionExecutionError.Code {
			data := err.Data.(TransactionExecutionErrorData)
			return FeeEstimate{}, httpHeader, MakeContractError(data.ExecutionError)
		}
		return FeeEstimate{}, httpHeader, err
	}

	return result.SimulatedTransactions[0].FeeEstimation, httpHeader, nil
}
