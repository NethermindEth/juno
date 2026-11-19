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
	senderAddress []felt.Address,
	finalityStatuses []TxnFinalityStatusWithoutL1,
) (SubscriptionID, *jsonrpc.Error) {
	wsConn, ok := jsonrpc.ConnFromContext(ctx)
	if !ok {
		return "", jsonrpc.Err(jsonrpc.MethodNotFound, nil)
	}

	sub, rpcErr := newReceiptsSubscriber(wsConn, senderAddress, finalityStatuses)
	if rpcErr != nil {
		return "", rpcErr
	}

	return h.subscribe(wsConn, sub)
}

// receiptsSubscriberState is touched only by its single subscription dispatch
// goroutine, so its pre_confirmed cursor is intentionally lock-free. Don't share it
// across goroutines.
type receiptsSubscriberState struct {
	conn               jsonrpc.Conn
	senders            []felt.Address
	preConfirmedCursor *rpccore.PreConfirmedCursor
}

func newReceiptsSubscriber(
	conn jsonrpc.Conn,
	senders []felt.Address,
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
		conn:               conn,
		senders:            senders,
		preConfirmedCursor: rpccore.NewPreConfirmedCursor(),
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
	s.preConfirmedCursor.Clear()
	return sendReorg(s.conn, reorg, id)
}

func (s *receiptsSubscriberState) onNewHead(
	_ context.Context,
	id string,
	_ *subscription,
	headWithBloom *core.WithBloom[*core.Block],
) error {
	head := headWithBloom.Value
	// Canonical blocks are published exactly once, so they bypass the pre_confirmed cursor.
	status := TxnFinalityStatusWithoutL1(TxnAcceptedOnL2)
	for receipt := range receiptsOf(head, s.senders, status, 0) {
		if err := sendTransactionReceipt(s.conn, &receipt, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *receiptsSubscriberState) onPreConfirmed(
	_ context.Context,
	id string,
	_ *subscription,
	preConfirmedWithBloom *core.WithBloom[*pending.PreConfirmed],
) error {
	preConfirmed := preConfirmedWithBloom.Value
	block := preConfirmed.GetBlock()
	status := TxnFinalityStatusWithoutL1(TxnPreConfirmed)
	// The pre_confirmed tip is re-published in full on every delta and only grows within a
	// round, so start after the transactions already processed. A same-height round
	// replacement changes BlockIdentifier and re-emits.
	from := s.preConfirmedCursor.Advance(
		block.Number,
		preConfirmed.BlockIdentifier,
		len(block.Transactions),
	)
	for receipt := range receiptsOf(block, s.senders, status, from) {
		if err := sendTransactionReceipt(s.conn, &receipt, id); err != nil {
			return err
		}
	}
	return nil
}

// receiptsOf yields each sender-matching receipt in the block from transaction index from,
// adapted to an RPC receipt.
func receiptsOf(
	block *core.Block,
	senders []felt.Address,
	finalityStatus TxnFinalityStatusWithoutL1,
	from int,
) iter.Seq[TransactionReceiptWithBlockInfo] {
	return func(yield func(TransactionReceiptWithBlockInfo) bool) {
		for i := from; i < len(block.Transactions); i++ {
			txn := block.Transactions[i]
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

func sendTransactionReceipt(
	wsConn jsonrpc.Conn,
	receipt *TransactionReceiptWithBlockInfo,
	id string,
) error {
	return sendResponse("starknet_subscriptionNewTransactionReceipts", wsConn, id, receipt)
}
