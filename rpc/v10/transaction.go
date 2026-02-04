package rpcv10

import (
	"context"
	"errors"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
	"github.com/NethermindEth/juno/starknet"
)

func (h *Handler) TransactionStatus(
	ctx context.Context,
	hash *felt.Felt,
) (rpcv9.TransactionStatus, *jsonrpc.Error) {
	receipt, txErr := h.TransactionReceiptByHash(hash)
	switch txErr {
	case nil:
		return rpcv9.TransactionStatus{
			Finality:      rpcv9.TxnStatus(receipt.FinalityStatus),
			Execution:     receipt.ExecutionStatus,
			FailureReason: receipt.RevertReason,
		}, nil
	case rpccore.ErrTxnHashNotFound:
		// Search pre-confirmed block for 'CANDIDATE' status
		var txStatus *starknet.TransactionStatus
		var err error
		preConfirmedB, err := h.PendingData()

		if err == nil {
			for _, txn := range preConfirmedB.GetCandidateTransaction() {
				if txn.Hash().Equal(hash) {
					txStatus = &starknet.TransactionStatus{FinalityStatus: starknet.Candidate}
					break
				}
			}
		}
		// Not Candidate
		if txStatus == nil {
			if h.feederClient == nil {
				break
			}

			txStatus, err = h.feederClient.Transaction(ctx, hash)
			if err != nil {
				return rpcv9.TransactionStatus{}, jsonrpc.Err(jsonrpc.InternalError, err.Error())
			}

			if txStatus.FinalityStatus == starknet.NotReceived && h.submittedTransactionsCache != nil {
				if h.submittedTransactionsCache.Contains(hash) {
					txStatus.FinalityStatus = starknet.Received
				}
			}
		}

		status, err := rpcv9.AdaptTransactionStatus(txStatus)
		if err != nil {
			if !errors.Is(err, rpcv9.ErrTransactionNotFound) {
				h.log.Error("Failed to adapt transaction status", utils.SugaredFields("err", err)...)
			}
			return rpcv9.TransactionStatus{}, rpccore.ErrTxnHashNotFound
		}
		return status, nil
	}
	return rpcv9.TransactionStatus{}, txErr
}

// TransactionReceiptByHash returns the receipt of a transaction identified by the given hash.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_api_openrpc.json#L222
func (h *Handler) TransactionReceiptByHash(
	hash *felt.Felt,
) (*rpcv9.TransactionReceipt, *jsonrpc.Error) {
	adaptedReceipt, rpcErr := h.getPendingTransactionReceipt(hash)
	if rpcErr == nil {
		return adaptedReceipt, nil
	}

	blockNumber, idx, err := h.bcReader.BlockNumberAndIndexByTxHash((*felt.TransactionHash)(hash))
	if err != nil {
		if !errors.Is(err, db.ErrKeyNotFound) {
			return nil, rpccore.ErrInternal.CloneWithData(err)
		}
		return nil, rpccore.ErrTxnHashNotFound
	}

	txn, err := h.bcReader.TransactionByBlockNumberAndIndex(blockNumber, idx)
	if err != nil {
		if !errors.Is(err, db.ErrKeyNotFound) {
			return nil, rpccore.ErrInternal.CloneWithData(err)
		}
		return nil, rpccore.ErrTxnHashNotFound
	}

	receipt, blockHash, err := h.bcReader.ReceiptByBlockNumberAndIndex(blockNumber, idx)
	if err != nil {
		if !errors.Is(err, db.ErrKeyNotFound) {
			return nil, rpccore.ErrInternal.CloneWithData(err)
		}
		return nil, rpccore.ErrTxnHashNotFound
	}

	l1H, jsonErr := h.l1Head()
	if jsonErr != nil {
		return nil, jsonErr
	}

	status := rpcv9.TxnAcceptedOnL2
	if isL1Verified(blockNumber, l1H) {
		status = rpcv9.TxnAcceptedOnL1
	}

	return rpcv9.AdaptReceiptWithBlockInfo(
		&receipt,
		txn,
		status,
		blockHash,
		blockNumber,
		false,
	), nil
}

// getPendingTransactionReceipt searches for a transaction receipt in the pending data.
// Returns the receipt if found, otherwise returns `rpccore.ErrTxnHashNotFound`.
func (h *Handler) getPendingTransactionReceipt(
	hash *felt.Felt,
) (*rpcv9.TransactionReceipt, *jsonrpc.Error) {
	pending, err := h.PendingData()
	if err != nil {
		return nil, rpccore.ErrTxnHashNotFound
	}

	receipt, parentHash, blockNumber, err := pending.ReceiptByHash(hash)
	if err != nil {
		return nil, rpccore.ErrTxnHashNotFound
	}

	txn, err := pending.TransactionByHash(hash)
	if err != nil {
		return nil, rpccore.ErrTxnHashNotFound
	}

	status := rpcv9.TxnPreConfirmed
	isPreLatest := false
	if parentHash != nil {
		// pre-latest block or pending block
		status = rpcv9.TxnAcceptedOnL2
		// If pending data is pre_confirmed receipt is coming from pre_latest
		isPreLatest = pending.Variant() == core.PreConfirmedBlockVariant
	}
	return rpcv9.AdaptReceiptWithBlockInfo(
		receipt,
		txn,
		status,
		nil,
		blockNumber,
		isPreLatest,
	), nil
}
