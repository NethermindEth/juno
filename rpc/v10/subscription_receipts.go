package rpcv10

import (
	"context"
	"fmt"
	"slices"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
)

// SubscribeNewTransactionReceipts creates a WebSocket stream which will fire events when
// new transaction receipts are created.
//
// The endpoint receives a vector of finality statuses.
// An event is fired for each finality status update.
// It is possible for receipts for pre-confirmed transactions to be received multiple times,
// or not at all.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/4e98e3684b50ee9e63b7eeea9412b6a2ed7494ec/api/starknet_ws_api.json#L186
//
//nolint:lll,nolintlint // url exceeds line limit, nolintlint because conflicting line limit with other lint rules
func (h *Handler) SubscribeNewTransactionReceipts(
	ctx context.Context,
	senderAddress []felt.Address,
	finalityStatuses []rpcv9.TxnFinalityStatusWithoutL1,
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

type receiptsSubscriberState struct {
	conn jsonrpc.Conn

	senderAddr []felt.Address
	sentCache  *rpccore.SubscriptionCache[rpcv9.SentReceipt, rpcv9.TxnFinalityStatusWithoutL1]
}

func newReceiptsSubscriber(
	conn jsonrpc.Conn,
	senderAddress []felt.Address,
	finalityStatuses []rpcv9.TxnFinalityStatusWithoutL1,
) (subscriber, *jsonrpc.Error) {
	if len(senderAddress) > rpccore.MaxEventFilterKeys {
		return subscriber{}, rpccore.ErrTooManyAddressesInFilter
	}

	if len(finalityStatuses) == 0 {
		finalityStatuses = []rpcv9.TxnFinalityStatusWithoutL1{
			rpcv9.TxnFinalityStatusWithoutL1(rpcv9.TxnAcceptedOnL2),
		}
	} else {
		finalityStatuses = utils.Set(finalityStatuses)
	}

	state := &receiptsSubscriberState{
		conn:       conn,
		senderAddr: senderAddress,
		sentCache:  rpccore.NewSubscriptionCache[rpcv9.SentReceipt, rpcv9.TxnFinalityStatusWithoutL1](),
	}

	s := subscriber{
		onReorg: state.onReorg,
	}

	if slices.Contains(finalityStatuses, rpcv9.TxnFinalityStatusWithoutL1(rpcv9.TxnAcceptedOnL2)) {
		s.onNewHead = state.onNewHead
		s.onPreLatest = state.onPreLatest
	}

	if slices.Contains(finalityStatuses, rpcv9.TxnFinalityStatusWithoutL1(rpcv9.TxnPreConfirmed)) {
		s.onPendingData = state.onPendingData
	}

	return s, nil
}

func (s *receiptsSubscriberState) onReorg(
	_ context.Context,
	id string,
	_ *subscription,
	reorg *sync.ReorgBlockRange,
) error {
	s.sentCache.Clear()
	return sendReorg(s.conn, reorg, id)
}

func (s *receiptsSubscriberState) onNewHead(
	_ context.Context,
	id string,
	_ *subscription,
	head *core.Block,
) error {
	return s.processBlock(
		id,
		head,
		rpcv9.TxnFinalityStatusWithoutL1(rpcv9.TxnAcceptedOnL2),
		false,
	)
}

func (s *receiptsSubscriberState) onPreLatest(
	_ context.Context,
	id string,
	_ *subscription,
	preLatest *core.PreLatest,
) error {
	return s.processBlock(
		id,
		preLatest.Block,
		rpcv9.TxnFinalityStatusWithoutL1(rpcv9.TxnAcceptedOnL2),
		true,
	)
}

func (s *receiptsSubscriberState) onPendingData(
	_ context.Context,
	id string,
	_ *subscription,
	pending core.PendingData,
) error {
	if pending.Variant() != core.PreConfirmedBlockVariant {
		return fmt.Errorf("unexpected pending data variant %v", pending.Variant())
	}

	return s.processBlock(
		id,
		pending.GetBlock(),
		rpcv9.TxnFinalityStatusWithoutL1(rpcv9.TxnPreConfirmed),
		false,
	)
}

func (s *receiptsSubscriberState) processBlock(
	id string,
	block *core.Block,
	finalityStatus rpcv9.TxnFinalityStatusWithoutL1,
	isPreLatest bool,
) error {
	for i, txn := range block.Transactions {
		if !filterTxBySender(txn, s.senderAddr) {
			continue
		}

		adaptedReceipt := rpcv9.AdaptReceiptWithBlockInfo(
			block.Receipts[i],
			txn,
			rpcv9.TxnFinalityStatus(finalityStatus),
			block.Hash,
			block.Number,
			isPreLatest,
		)

		sentReceipt := rpcv9.SentReceipt{
			TransactionHash:  *adaptedReceipt.Hash,
			TransactionIndex: i,
		}

		if !s.sentCache.ShouldSend(block.Number, &sentReceipt, &finalityStatus) {
			continue
		}

		s.sentCache.Put(block.Number, &sentReceipt, &finalityStatus)

		if err := sendTransactionReceipt(s.conn, adaptedReceipt, id); err != nil {
			return err
		}
	}
	return nil
}

func sendTransactionReceipt(w jsonrpc.Conn, receipt *rpcv9.TransactionReceipt, id string) error {
	return sendResponse("starknet_subscriptionNewTransactionReceipts", w, id, receipt)
}
