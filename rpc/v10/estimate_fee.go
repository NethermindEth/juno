package rpcv10

import (
	"net/http"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
)

/*
***************************************************

	Estimate Fee Handlers

****************************************************
*/
func (h *Handler) EstimateFee(
	broadcastedTxns BroadcastedTransactionInputs,
	estimateFlags []EstimateFlag,
	id *rpcv9.BlockID,
) ([]rpcv9.FeeEstimate, http.Header, *jsonrpc.Error) {
	simulationFlags := make([]SimulationFlag, 0, len(estimateFlags)+1)
	for _, flag := range estimateFlags {
		simulationFlag, err := flag.ToSimulationFlag()
		if err != nil {
			return nil, nil, jsonrpc.Err(jsonrpc.InvalidParams, err.Error())
		}
		simulationFlags = append(simulationFlags, simulationFlag)
	}

	txnResults, httpHeader, err := h.simulateTransactions(
		id,
		broadcastedTxns.Data,
		append(simulationFlags, SkipFeeChargeFlag),
		true,
		true,
	)
	if err != nil {
		return nil, httpHeader, err
	}

	simulatedTransactions := txnResults.SimulatedTransactions
	feeEstimates := make([]rpcv9.FeeEstimate, len(simulatedTransactions))
	for i := range feeEstimates {
		feeEstimates[i] = simulatedTransactions[i].FeeEstimation
	}

	return feeEstimates, httpHeader, nil
}

func (h *Handler) EstimateMessageFee(
	msg *rpcv6.MsgFromL1, id *rpcv9.BlockID,
) (rpcv9.FeeEstimate, http.Header, *jsonrpc.Error) {
	calldata := make([]*felt.Felt, len(msg.Payload)+1)
	// msg.From needs to be the first element
	calldata[0] = felt.NewFromBytes[felt.Felt](msg.From.Bytes())
	for i := range msg.Payload {
		calldata[i+1] = &msg.Payload[i]
	}

	state, closer, rpcErr := h.stateByBlockID(id)
	if rpcErr != nil {
		return rpcv9.FeeEstimate{}, nil, rpcErr
	}
	defer h.callAndLogErr(closer, "Failed to close state in starknet_estimateMessageFee")

	if _, err := state.ContractClassHash(&msg.To); err != nil {
		return rpcv9.FeeEstimate{}, nil, rpccore.ErrContractNotFound
	}

	tx := rpcv9.BroadcastedTransaction{
		Transaction: rpcv9.Transaction{
			Type:               rpcv9.TxnL1Handler,
			ContractAddress:    &msg.To,
			EntryPointSelector: &msg.Selector,
			CallData:           &calldata,
			Version:            &felt.Zero, // Needed for transaction hash calculation.
			Nonce:              &felt.Zero, // Needed for transaction hash calculation.
		},
		// Needed to marshal to blockifier type.
		// Must be greater than zero to successfully execute transaction.
		PaidFeeOnL1: felt.NewFromUint64[felt.Felt](1),
	}

	bcTxn := [1]rpcv9.BroadcastedTransaction{tx}
	estimates, httpHeader, err := h.EstimateFee(
		rpccore.LimitSlice[rpcv9.BroadcastedTransaction, rpccore.SimulationLimit]{Data: bcTxn[:]},
		nil,
		id,
	)
	if err != nil {
		if err.Code == rpccore.ErrTransactionExecutionError.Code {
			data := err.Data.(rpcv9.TransactionExecutionErrorData)
			return rpcv9.FeeEstimate{}, httpHeader, rpcv9.MakeContractError(data.ExecutionError)
		}
		return rpcv9.FeeEstimate{}, httpHeader, err
	}
	return estimates[0], httpHeader, nil
}
