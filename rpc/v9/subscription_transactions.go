package rpcv9

import (
	"context"
	"fmt"
	"slices"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
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
// It is possible for events for pre-confirmed and
// candidate transactions to be received multiple times,
// or not at all.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/4e98e3684b50ee9e63b7eeea9412b6a2ed7494ec/api/starknet_ws_api.json#L257
//
//nolint:lll,nolintlint // url exceeds line limit, nolintlint because conflicting line limit with other lint rules
func (h *Handler) SubscribeNewTransactions(
	ctx context.Context,
	finalityStatus []TxnStatusWithoutL1,
	senderAddr []felt.Felt,
) (SubscriptionID, *jsonrpc.Error) {
	w, ok := jsonrpc.ConnFromContext(ctx)
	if !ok {
		return "", jsonrpc.Err(jsonrpc.MethodNotFound, nil)
	}

	sub, rpcErr := newTransactionsSubscriber(w, finalityStatus, senderAddr)
	if rpcErr != nil {
		return "", rpcErr
	}

	return h.subscribe(ctx, w, sub)
}

type SubscriptionNewTransaction struct {
	Transaction
	FinalityStatus TxnStatusWithoutL1 `json:"finality_status"`
}

type transactionsSubscriberState struct {
	conn           jsonrpc.Conn
	senderAddr     []felt.Felt
	finalityStatus []TxnStatusWithoutL1
	sentCache      *rpccore.SubscriptionCache[felt.TransactionHash, TxnStatusWithoutL1]
}

func newTransactionsSubscriber(
	conn jsonrpc.Conn,
	finalityStatus []TxnStatusWithoutL1,
	senderAddr []felt.Felt,
) (subscriber, *jsonrpc.Error) {
	if len(senderAddr) > rpccore.MaxEventFilterKeys {
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
		conn:           conn,
		senderAddr:     senderAddr,
		finalityStatus: finalityStatus,
		sentCache:      rpccore.NewSubscriptionCache[felt.TransactionHash, TxnStatusWithoutL1](),
	}

	s := subscriber{
		onReorg: state.onReorg,
	}

	if slices.Contains(state.finalityStatus, TxnStatusWithoutL1(TxnStatusAcceptedOnL2)) {
		s.onNewHead = state.onNewHead
		s.onPreLatest = state.onPreLatest
	}

	if slices.ContainsFunc(
		state.finalityStatus,
		func(status TxnStatusWithoutL1) bool {
			return status == TxnStatusWithoutL1(TxnStatusPreConfirmed) ||
				status == TxnStatusWithoutL1(TxnStatusCandidate)
		}) {
		s.onPendingData = state.onPendingData
	}

	return s, nil
}

func (s *transactionsSubscriberState) onReorg(
	_ context.Context,
	id string,
	_ *subscription,
	reorg *sync.ReorgBlockRange,
) error {
	s.sentCache.Clear()
	return sendReorg(s.conn, reorg, id)
}

func (s *transactionsSubscriberState) onNewHead(
	_ context.Context,
	id string,
	_ *subscription,
	head *core.Block,
) error {
	return s.processBlock(
		id,
		head,
		TxnStatusWithoutL1(TxnStatusAcceptedOnL2),
	)
}

func (s *transactionsSubscriberState) onPreLatest(
	_ context.Context,
	id string,
	_ *subscription,
	preLatest *core.PreLatest,
) error {
	return s.processBlock(
		id,
		preLatest.Block,
		TxnStatusWithoutL1(TxnStatusAcceptedOnL2),
	)
}

func (s *transactionsSubscriberState) onPendingData(
	_ context.Context,
	id string,
	_ *subscription,
	pending core.PendingData,
) error {
	if pending.Variant() != core.PreConfirmedBlockVariant {
		return fmt.Errorf("unexpected pending data variant %v", pending.Variant())
	}

	if slices.Contains(s.finalityStatus, TxnStatusWithoutL1(TxnStatusPreConfirmed)) {
		if err := s.processBlock(
			id,
			pending.GetBlock(),
			TxnStatusWithoutL1(TxnStatusPreConfirmed),
		); err != nil {
			return err
		}
	}

	if slices.Contains(s.finalityStatus, TxnStatusWithoutL1(TxnStatusCandidate)) {
		return s.processCandidateTransactions(id, pending)
	}

	return nil
}

func (s *transactionsSubscriberState) processBlock(
	id string,
	b *core.Block,
	status TxnStatusWithoutL1,
) error {
	for _, txn := range b.Transactions {
		if !filterTxBySender(txn, s.senderAddr) {
			continue
		}

		if err := s.sendWithoutDuplicate(
			id,
			b.Number,
			txn,
			status,
		); err != nil {
			return err
		}
	}
	return nil
}

func (s *transactionsSubscriberState) processCandidateTransactions(
	id string,
	preConfirmed core.PendingData,
) error {
	blockNumber := preConfirmed.GetBlock().Number
	for _, txn := range preConfirmed.GetCandidateTransaction() {
		if !filterTxBySender(txn, s.senderAddr) {
			continue
		}

		if err := s.sendWithoutDuplicate(
			id,
			blockNumber,
			txn,
			TxnStatusWithoutL1(TxnStatusCandidate),
		); err != nil {
			return err
		}
	}
	return nil
}

func (s *transactionsSubscriberState) sendWithoutDuplicate(
	id string,
	blockNumber uint64,
	txn core.Transaction,
	finalityStatus TxnStatusWithoutL1,
) error {
	txHash := felt.TransactionHash(*txn.Hash())
	if !s.sentCache.ShouldSend(
		blockNumber,
		&txHash,
		&finalityStatus,
	) {
		return nil
	}

	s.sentCache.Put(blockNumber, &txHash, &finalityStatus)

	response := SubscriptionNewTransaction{
		Transaction:    *AdaptTransaction(txn),
		FinalityStatus: finalityStatus,
	}

	return sendTransaction(s.conn, &response, id)
}

func sendTransaction(w jsonrpc.Conn, result *SubscriptionNewTransaction, id string) error {
	return sendResponse("starknet_subscriptionNewTransaction", w, id, result)
}
