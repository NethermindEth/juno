package rpcv10

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/pending"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/sync"
	"go.uber.org/zap"
)

type SubscriptionID string

func (h *Handler) unsubscribe(sub *subscription, id string) {
	sub.cancel()
	h.subscriptions.Delete(id)
}

type on[T any] func(ctx context.Context, id string, sub *subscription, event T) error

type subscriber struct {
	onStart               on[any]
	onReorg               on[*sync.ReorgBlockRange]
	onNewHead             on[*core.Block]
	onPreConfirmed        on[*pending.PreConfirmed]
	onL1Head              on[*core.L1Head]
	onReceivedTransaction on[core.Transaction]
}

func getSubscription[T any](callback on[T], feed *feed.Feed[T]) (*feed.Subscription[T], <-chan T) {
	if callback != nil && feed != nil {
		sub := feed.SubscribeKeepLast()
		recv := sub.Recv()
		return sub, recv
	}
	return nil, nil
}

func unsubscribeFeedSubscription[T any](sub *feed.Subscription[T]) {
	if sub != nil {
		sub.Unsubscribe()
	}
}

//nolint:gocyclo // select statement with multiple subscription cases
func (h *Handler) subscribe(
	wsConn jsonrpc.Conn,
	subscriber subscriber,
) (SubscriptionID, *jsonrpc.Error) {
	id := h.idgen()
	//nolint:gosec // G118: cancel called in unsubscribe()
	subscriptionCtx, subscriptionCtxCancel := context.WithCancel(wsConn.Context())
	sub := &subscription{
		cancel: subscriptionCtxCancel,
		conn:   wsConn,
	}
	h.subscriptions.Store(id, sub)

	reorgSub, reorgRecv := getSubscription(subscriber.onReorg, h.reorgs)
	newHeadsSub, newHeadsRecv := getSubscription(subscriber.onNewHead, h.newHeads)
	preConfirmedSub, preConfirmedRecv := getSubscription(
		subscriber.onPreConfirmed, h.preConfirmedFeed,
	)
	l1HeadSub, l1HeadRecv := getSubscription(subscriber.onL1Head, h.l1Heads)
	receivedTransactionSub, receivedTransactionRecv := getSubscription(
		subscriber.onReceivedTransaction,
		h.receivedTransactionFeed,
	)

	sub.wg.Go(func() {
		defer func() {
			h.unsubscribe(sub, id)
			unsubscribeFeedSubscription(reorgSub)
			unsubscribeFeedSubscription(l1HeadSub)
			unsubscribeFeedSubscription(newHeadsSub)
			unsubscribeFeedSubscription(preConfirmedSub)
			unsubscribeFeedSubscription(receivedTransactionSub)
		}()

		if subscriber.onStart != nil {
			if err := subscriber.onStart(subscriptionCtx, id, sub, nil); err != nil {
				h.logger.Warn("Error starting subscription", zap.Error(err))
				return
			}
		}

		for {
			select {
			case <-subscriptionCtx.Done():
				return

			case reorg := <-reorgRecv:
				if err := subscriber.onReorg(subscriptionCtx, id, sub, reorg); err != nil {
					h.logger.Warn("Error on reorg", zap.String("id", id), zap.Error(err))
					return
				}

			case l1Head := <-l1HeadRecv:
				if err := subscriber.onL1Head(subscriptionCtx, id, sub, l1Head); err != nil {
					h.logger.Warn("Error on l1 head", zap.String("id", id), zap.Error(err))
					return
				}

			case head := <-newHeadsRecv:
				if err := subscriber.onNewHead(subscriptionCtx, id, sub, head); err != nil {
					h.logger.Warn("Error on new head", zap.String("id", id), zap.Error(err))
					return
				}

			case preConfirmed := <-preConfirmedRecv:
				err := subscriber.onPreConfirmed(subscriptionCtx, id, sub, preConfirmed)
				if err != nil {
					h.logger.Warn("Error on pre confirmed", zap.String("id", id), zap.Error(err))
					return
				}

			case transaction := <-receivedTransactionRecv:
				err := subscriber.onReceivedTransaction(subscriptionCtx, id, sub, transaction)
				if err != nil {
					h.logger.Warn("Error on received transaction",
						zap.String("id", id),
						zap.Error(err),
					)
					return
				}
			}
		}
	})

	return SubscriptionID(id), nil
}

// filterTxBySender checks if the transaction is included in the sender address list.
// If the sender address list is empty, it will return true by default.
// If the sender address list is not empty, it will check if the transaction is an Invoke or
// Declare transaction and if the sender address is in the list.
// For other transaction types, it will by default return false.
func filterTxBySender(txn core.Transaction, senderAddr []felt.Address) bool {
	if len(senderAddr) == 0 {
		return true
	}

	switch t := txn.(type) {
	case *core.InvokeTransaction:
		for _, addr := range senderAddr {
			// todo: remove the cast to felt.Felt
			if t.SenderAddress.Equal((*felt.Felt)(&addr)) {
				return true
			}
		}
	case *core.DeclareTransaction:
		for _, addr := range senderAddr {
			// todo: remove the cast to felt.Felt
			if t.SenderAddress.Equal((*felt.Felt)(&addr)) {
				return true
			}
		}
	}

	return false
}

// resolveBlockRange returns the start and latest block numbers based on the blockID.
func (h *Handler) resolveBlockRange(
	blockID *SubscriptionBlockID,
) (uint64, uint64, *jsonrpc.Error) {
	latestBlock, err := h.bcReader.Height()
	if err != nil {
		return 0, 0, rpccore.ErrInternal.CloneWithData(err.Error())
	}

	if blockID == nil || blockID.IsLatest() {
		return latestBlock, latestBlock, nil
	}

	var startBlock uint64
	if blockID.IsHash() {
		startBlock, err = h.bcReader.BlockNumberByHash(blockID.Hash())
		if err != nil {
			if errors.Is(err, db.ErrKeyNotFound) {
				return 0, 0, rpccore.ErrBlockNotFound
			}
			return 0, 0, rpccore.ErrInternal.CloneWithData(err.Error())
		}
	} else {
		startBlock = blockID.Number()
		if startBlock > latestBlock {
			return 0, 0, rpccore.ErrBlockNotFound
		}
	}

	tooManyBlocks := latestBlock >= rpccore.MaxBlocksBack &&
		startBlock <= latestBlock-rpccore.MaxBlocksBack
	if tooManyBlocks {
		return 0, 0, rpccore.ErrTooManyBlocksBack
	}

	return startBlock, latestBlock, nil
}

func (h *Handler) Unsubscribe(ctx context.Context, id string) (bool, *jsonrpc.Error) {
	wsConn, ok := jsonrpc.ConnFromContext(ctx)
	if !ok {
		return false, jsonrpc.Err(jsonrpc.MethodNotFound, nil)
	}
	sub, ok := h.subscriptions.Load(id)
	if !ok {
		return false, rpccore.ErrInvalidSubscriptionID
	}

	subs := sub.(*subscription)
	if !subs.conn.Equal(wsConn) {
		return false, rpccore.ErrInvalidSubscriptionID
	}

	subs.cancel()
	subs.wg.Wait() // Let the subscription finish before responding.
	h.subscriptions.Delete(id)
	return true, nil
}

func sendReorg(wsConn jsonrpc.Conn, reorg *sync.ReorgBlockRange, id string) error {
	return sendResponse("starknet_subscriptionReorg", wsConn, id, &ReorgEvent{
		StartBlockHash: reorg.StartBlockHash,
		StartBlockNum:  reorg.StartBlockNum,
		EndBlockHash:   reorg.EndBlockHash,
		EndBlockNum:    reorg.EndBlockNum,
	})
}

func sendResponse[T any](method string, wsConn jsonrpc.Conn, id string, result T) error {
	resp, err := json.Marshal(SubscriptionResponse[T]{
		Version: "2.0",
		Method:  method,
		Params: SubscriptionParams[T]{
			Result:         result,
			SubscriptionID: id,
		},
	})
	if err != nil {
		return err
	}
	_, err = wsConn.Write(resp)
	return err
}
