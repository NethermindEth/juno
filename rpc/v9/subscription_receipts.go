package rpcv9

import (
	"context"
	"iter"
	"slices"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/pending"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
)

// SubscribeNewTransactionReceipts creates a WebSocket stream which will fire
// events when new transaction receipts are created.
// The endpoint receives a vector of finality statuses.
// An event is fired for each finality status update.
// It is possible for receipts for pre-confirmed transactions to be received
// multiple times, or not at all.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/c2e93098b9c2ca0423b7f4d15b201f52f22d8c36/api/starknet_ws_api.json#L193
//
//nolint:lll,nolintlint // url exceeds line limit, nolintlint because conflicting line limit with other lint rules
func (h *Handler) SubscribeNewTransactionReceipts(
	ctx context.Context,
	senderAddress []felt.Felt,
	finalityStatuses []TxnFinalityStatusWithoutL1,
) (SubscriptionID, *jsonrpc.Error) {
	w, ok := jsonrpc.ConnFromContext(ctx)
	if !ok {
		return "", jsonrpc.Err(jsonrpc.MethodNotFound, nil)
	}

	sub, rpcErr := newReceiptsSubscriber(w, senderAddress, finalityStatuses)
	if rpcErr != nil {
		return "", rpcErr
	}

	return h.subscribe(ctx, w, sub)
}

// receiptsSubscriberState is touched only by its single subscription dispatch
// goroutine, so its deduper is intentionally lock-free. Don't share it across goroutines.
type receiptsSubscriberState struct {
	conn    jsonrpc.Conn
	senders []felt.Felt
	deduper *rpccore.PreConfirmedDeduper[felt.TransactionHash]
}

func newReceiptsSubscriber(
	conn jsonrpc.Conn,
	senders []felt.Felt,
	finalityStatuses []TxnFinalityStatusWithoutL1,
) (subscriber, *jsonrpc.Error) {
	if len(senders) > rpccore.MaxEventFilterKeys {
		return subscriber{}, rpccore.ErrTooManyAddressesInFilter
	}

	if len(finalityStatuses) == 0 {
		finalityStatuses = []TxnFinalityStatusWithoutL1{
			TxnFinalityStatusWithoutL1(TxnAcceptedOnL2),
		}
	} else {
		finalityStatuses = utils.Set(finalityStatuses)
	}

	state := &receiptsSubscriberState{
		conn:    conn,
		senders: senders,
		deduper: rpccore.NewPreConfirmedDeduper[felt.TransactionHash](),
	}

	s := subscriber{
		onReorg: state.onReorg,
	}

	if slices.Contains(finalityStatuses, TxnFinalityStatusWithoutL1(TxnAcceptedOnL2)) {
		s.onNewHead = state.onNewHead
	}

	if slices.Contains(finalityStatuses, TxnFinalityStatusWithoutL1(TxnPreConfirmed)) {
		s.onPreConfirmed = state.onPreConfirmed
	}

	return s, nil
}

func (s *receiptsSubscriberState) onReorg(
	_ context.Context,
	id string,
	_ *subscription,
	reorg *sync.ReorgBlockRange,
) error {
	s.deduper.Clear()
	return sendReorg(s.conn, reorg, id)
}

func (s *receiptsSubscriberState) onNewHead(
	_ context.Context,
	id string,
	_ *subscription,
	head *core.Block,
) error {
	// Canonical blocks are published exactly once, so they bypass the deduper.
	for receipt := range receiptsOf(head, s.senders, TxnFinalityStatusWithoutL1(TxnAcceptedOnL2)) {
		if err := sendTransactionReceipt(s.conn, receipt, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *receiptsSubscriberState) onPreConfirmed(
	_ context.Context,
	id string,
	_ *subscription,
	preConfirmed *pending.PreConfirmed,
) error {
	block := preConfirmed.GetBlock()
	status := TxnFinalityStatusWithoutL1(TxnPreConfirmed)
	// The pre_confirmed tip is re-published in full on every delta; skip already-sent
	// receipts. A same-height round replacement changes BlockIdentifier and re-emits.
	for receipt := range receiptsOf(block, s.senders, status) {
		shouldSend := s.deduper.MarkSent(
			block.Number,
			preConfirmed.BlockIdentifier,
			(*felt.TransactionHash)(receipt.Hash),
		)
		if !shouldSend {
			continue
		}

		if err := sendTransactionReceipt(s.conn, receipt, id); err != nil {
			return err
		}
	}
	return nil
}

// receiptsOf yields each sender-matching receipt in the block, adapted to an RPC receipt.
func receiptsOf(
	block *core.Block,
	senders []felt.Felt,
	finalityStatus TxnFinalityStatusWithoutL1,
) iter.Seq[*TransactionReceipt] {
	return func(yield func(*TransactionReceipt) bool) {
		for i, txn := range block.Transactions {
			if !filterTxBySender(txn, senders) {
				continue
			}

			adaptedReceipt := AdaptReceiptWithBlockInfo(
				block.Receipts[i],
				txn,
				TxnFinalityStatus(finalityStatus),
				block.Hash,
				block.Number,
			)

			if !yield(adaptedReceipt) {
				return
			}
		}
	}
}

func sendTransactionReceipt(w jsonrpc.Conn, receipt *TransactionReceipt, id string) error {
	return sendResponse("starknet_subscriptionNewTransactionReceipts", w, id, receipt)
}
