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

// SubscribeNewTransactions Creates a WebSocket stream which will fire events when
// new transaction are created.
//
// The endpoint receives a vector of finality statuses.
// An event is fired for each finality status update.
// It is possible for events for pre-confirmed transactions to be received
// multiple times, or not at all.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/c2e93098b9c2ca0423b7f4d15b201f52f22d8c36/api/starknet_ws_api.json#L264
//
//nolint:lll,nolintlint // url exceeds line limit, nolintlint because conflicting line limit with other lint rules
func (h *Handler) SubscribeNewTransactions(
	ctx context.Context,
	finalityStatus []TxnStatusWithoutL1,
	senderAddr []felt.Address,
) (SubscriptionID, *jsonrpc.Error) {
	wsConn, ok := jsonrpc.ConnFromContext(ctx)
	if !ok {
		return "", jsonrpc.Err(jsonrpc.MethodNotFound, nil)
	}

	sub, rpcErr := newTransactionsSubscriber(wsConn, finalityStatus, senderAddr)
	if rpcErr != nil {
		return "", rpcErr
	}

	return h.subscribe(wsConn, sub)
}

type SubscriptionNewTransaction struct {
	Transaction
	FinalityStatus TxnStatusWithoutL1 `json:"finality_status"`
}

// transactionsSubscriberState is touched only by its single subscription dispatch
// goroutine, so its pre_confirmed cursor is intentionally lock-free. Don't share it
// across goroutines.
type transactionsSubscriberState struct {
	conn               jsonrpc.Conn
	senders            []felt.Address
	finalityStatus     []TxnStatusWithoutL1
	preConfirmedCursor *rpccore.PreConfirmedCursor
}

func newTransactionsSubscriber(
	conn jsonrpc.Conn,
	finalityStatus []TxnStatusWithoutL1,
	senders []felt.Address,
) (subscriber, *jsonrpc.Error) {
	if len(senders) > rpccore.MaxEventFilterKeys {
		return subscriber{}, rpccore.ErrTooManyAddressesInFilter
	}

	if len(finalityStatus) == 0 {
		finalityStatus = []TxnStatusWithoutL1{
			TxnStatusWithoutL1(TxnStatusAcceptedOnL2),
		}
	} else {
		finalityStatus = utils.Set(finalityStatus)
	}

	state := &transactionsSubscriberState{
		conn:               conn,
		senders:            senders,
		finalityStatus:     finalityStatus,
		preConfirmedCursor: rpccore.NewPreConfirmedCursor(),
	}

	s := subscriber{
		onReorg:    state.onReorg,
		filterSize: int64(len(senders)) * rpccore.SenderAddressSize,
	}

	if slices.Contains(state.finalityStatus, TxnStatusWithoutL1(TxnStatusAcceptedOnL2)) {
		s.onNewHead = state.onNewHead
	}

	if slices.Contains(state.finalityStatus, TxnStatusWithoutL1(TxnStatusPreConfirmed)) {
		s.onPreConfirmed = state.onPreConfirmed
	}

	if slices.Contains(state.finalityStatus, TxnStatusWithoutL1(TxnStatusReceived)) {
		s.onReceivedTransaction = state.onReceivedTransaction
	}

	return s, nil
}

func (s *transactionsSubscriberState) onReorg(
	_ context.Context,
	id string,
	_ *subscription,
	reorg *sync.ReorgBlockRange,
) error {
	s.preConfirmedCursor.Clear()
	return sendReorg(s.conn, reorg, id)
}

func (s *transactionsSubscriberState) onNewHead(
	_ context.Context,
	id string,
	_ *subscription,
	headWithBloom *core.WithBloom[*core.Block],
) error {
	head := headWithBloom.Value
	// Canonical blocks are published exactly once, so they bypass the pre_confirmed cursor.
	for txn := range transactionsOf(head, s.senders, TxnStatusWithoutL1(TxnStatusAcceptedOnL2), 0) {
		if err := sendTransaction(s.conn, txn, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *transactionsSubscriberState) onPreConfirmed(
	_ context.Context,
	id string,
	_ *subscription,
	preConfirmedWithBloom *core.WithBloom[*pending.PreConfirmed],
) error {
	preConfirmed := preConfirmedWithBloom.Value
	block := preConfirmed.GetBlock()
	status := TxnStatusWithoutL1(TxnStatusPreConfirmed)
	// The pre_confirmed tip is re-published in full on every delta and only grows within a
	// round, so start after the transactions already processed. A same-height round
	// replacement changes BlockIdentifier and re-emits.
	from := s.preConfirmedCursor.Advance(
		block.Number,
		preConfirmed.BlockIdentifier,
		len(block.Transactions),
	)
	for txn := range transactionsOf(block, s.senders, status, from) {
		if err := sendTransaction(s.conn, txn, id); err != nil {
			return err
		}
	}
	return nil
}

// transactionsOf yields each sender-matching transaction in the block from index from,
// adapted to an RPC subscription transaction with the given finality status.
func transactionsOf(
	b *core.Block,
	senders []felt.Address,
	finalityStatus TxnStatusWithoutL1,
	from int,
) iter.Seq[*SubscriptionNewTransaction] {
	return func(yield func(*SubscriptionNewTransaction) bool) {
		for _, txn := range b.Transactions[from:] {
			if !filterTxBySender(txn, senders) {
				continue
			}

			response := &SubscriptionNewTransaction{
				Transaction:    AdaptTransaction(txn),
				FinalityStatus: finalityStatus,
			}

			if !yield(response) {
				return
			}
		}
	}
}

func (s *transactionsSubscriberState) onReceivedTransaction(
	_ context.Context,
	id string,
	_ *subscription,
	txn core.Transaction,
) error {
	if !filterTxBySender(txn, s.senders) {
		return nil
	}

	response := SubscriptionNewTransaction{
		Transaction:    AdaptTransaction(txn),
		FinalityStatus: TxnStatusWithoutL1(TxnStatusReceived),
	}

	return sendTransaction(s.conn, &response, id)
}

func sendTransaction(wsConn jsonrpc.Conn, result *SubscriptionNewTransaction, id string) error {
	return sendResponse("starknet_subscriptionNewTransaction", wsConn, id, result)
}
