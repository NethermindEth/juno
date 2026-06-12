package rpcv10

import (
	"context"
	"net/http"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/rpc/rpccore"
)

// MsgFromL1 represents a message sent from L1 to L2.
type MsgFromL1 struct {
	// The address of the L1 contract sending the message.
	From eth.Address `json:"from_address" validate:"required"`
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
	simulationFlags := make([]SimulationFlag, 0, len(estimateFlags))
	for _, flag := range estimateFlags {
		simulationFlag, err := flag.ToSimulationFlag()
		if err != nil {
			return nil, nil, jsonrpc.Err(jsonrpc.InvalidParams, err.Error())
		}
		simulationFlags = append(simulationFlags, simulationFlag)
	}

	txnResults, httpHeader, err := h.simulateBroadcastedTransactions(
		ctx,
		id,
		broadcastedTxns.Data,
		simulationFlags,
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

	// In order to estimate the message fee, since there's no "message fee" transaction,
	// we wrap the message in a L1_HANDLER transaction.
	l1Handler := core.L1HandlerTransaction{
		ContractAddress:    &msg.To,
		EntryPointSelector: &msg.Selector,
		CallData:           calldata,
		// Needed for L1_HANDLER transaction hash calculation. Every transaction to be
		// simulated must contain a valid tx hash; since we are just estimating the fees,
		// we can set the remaining required fields to zero, as we don't care about the
		// final tx hash.
		Version: (*core.TransactionVersion)(&felt.Zero),
		Nonce:   &felt.Zero,
	}
	txnHash, hashErr := core.TransactionHash(&l1Handler, h.bcReader.Network())
	if hashErr != nil {
		return FeeEstimate{}, nil, rpccore.ErrInternal.CloneWithData(hashErr.Error())
	}
	l1Handler.TransactionHash = &txnHash

	result, httpHeader, err := h.simulateTransactions(
		id,
		[]core.Transaction{&l1Handler},
		nil,  // classes
		nil,  // simulation flags
		true, // err on revert
		true, // is estimate fee
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
