package rpcv9

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/sync"
	"go.uber.org/zap"
)

type SubscriptionResponse struct {
	Version string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

// As per the spec, this is the same as BlockID, but without `pre_confirmed` and `l1_accepted`
type SubscriptionBlockID BlockID

func (b *SubscriptionBlockID) Type() blockIDType {
	return b.typeID
}

func (b *SubscriptionBlockID) IsLatest() bool {
	return b.typeID == latest
}

func (b *SubscriptionBlockID) IsHash() bool {
	return b.typeID == hash
}

func (b *SubscriptionBlockID) IsNumber() bool {
	return b.typeID == number
}

func (b *SubscriptionBlockID) Hash() *felt.Felt {
	return (*BlockID)(b).Hash()
}

func (b *SubscriptionBlockID) Number() uint64 {
	return (*BlockID)(b).Number()
}

func (b *SubscriptionBlockID) UnmarshalJSON(data []byte) error {
	blockID := (*BlockID)(b)
	err := blockID.UnmarshalJSON(data)
	if err != nil {
		return err
	}

	if blockID.IsPreConfirmed() {
		return errors.New("subscription block id cannot be pre_confirmed")
	}

	if blockID.IsL1Accepted() {
		return errors.New("subscription block id cannot be l1_accepted")
	}

	return nil
}

type SubscriptionID string

func (h *Handler) unsubscribe(sub *subscription, id string) {
	sub.cancel()
	h.subscriptions.Delete(id)
}

type on[T any] func(ctx context.Context, id string, sub *subscription, event T) error

type subscriber struct {
	onStart       on[any]
	onReorg       on[*sync.ReorgBlockRange]
	onNewHead     on[*core.Block]
	onPendingData on[core.PendingData]
	onL1Head      on[*core.L1Head]
	onPreLatest   on[*core.PreLatest]
}

func getSubscription[T any](callback on[T], feed *feed.Feed[T]) (*feed.Subscription[T], <-chan T) {
	if callback != nil {
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

func (h *Handler) subscribe(
	ctx context.Context,
	w jsonrpc.Conn,
	subscriber subscriber,
) (SubscriptionID, *jsonrpc.Error) {
	id := h.idgen()
	subscriptionCtx, subscriptionCtxCancel := context.WithCancel(ctx)
	sub := &subscription{
		cancel: subscriptionCtxCancel,
		conn:   w,
	}
	h.subscriptions.Store(id, sub)

	reorgSub, reorgRecv := getSubscription(subscriber.onReorg, h.reorgs)
	newHeadsSub, newHeadsRecv := getSubscription(subscriber.onNewHead, h.newHeads)
	pendingDataSub, pendingRecv := getSubscription(subscriber.onPendingData, h.pendingData)
	l1HeadSub, l1HeadRecv := getSubscription(subscriber.onL1Head, h.l1Heads)
	preLatestSub, preLatestRecv := getSubscription(subscriber.onPreLatest, h.preLatestFeed)

	sub.wg.Go(func() {
		defer func() {
			h.unsubscribe(sub, id)
			unsubscribeFeedSubscription(reorgSub)
			unsubscribeFeedSubscription(l1HeadSub)
			unsubscribeFeedSubscription(newHeadsSub)
			unsubscribeFeedSubscription(pendingDataSub)
			unsubscribeFeedSubscription(preLatestSub)
		}()

		if subscriber.onStart != nil {
			if err := subscriber.onStart(subscriptionCtx, id, sub, nil); err != nil {
				h.log.Warn("Error starting subscription", zap.Error(err))
				return
			}
		}

		for {
			select {
			case <-subscriptionCtx.Done():
				return
			case reorg := <-reorgRecv:
				if err := subscriber.onReorg(subscriptionCtx, id, sub, reorg); err != nil {
					h.log.Warn("Error on reorg", zap.String("id", id), zap.Error(err))
					return
				}
			case l1Head := <-l1HeadRecv:
				if err := subscriber.onL1Head(subscriptionCtx, id, sub, l1Head); err != nil {
					h.log.Warn("Error on l1 head", zap.String("id", id), zap.Error(err))
					return
				}
			case head := <-newHeadsRecv:
				if err := subscriber.onNewHead(subscriptionCtx, id, sub, head); err != nil {
					h.log.Warn("Error on new head", zap.String("id", id), zap.Error(err))
					return
				}
			case pending := <-pendingRecv:
				if err := subscriber.onPendingData(subscriptionCtx, id, sub, pending); err != nil {
					h.log.Warn("Error on pending data", zap.String("id", id), zap.Error(err))
					return
				}
			case preLatest := <-preLatestRecv:
				if err := subscriber.onPreLatest(subscriptionCtx, id, sub, preLatest); err != nil {
					h.log.Warn("Error on  preLatest", zap.String("id", id), zap.Error(err))
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
func filterTxBySender(txn core.Transaction, senderAddr []felt.Felt) bool {
	if len(senderAddr) == 0 {
		return true
	}

	switch t := txn.(type) {
	case *core.InvokeTransaction:
		for _, addr := range senderAddr {
			if t.SenderAddress.Equal(&addr) {
				return true
			}
		}
	case *core.DeclareTransaction:
		for _, addr := range senderAddr {
			if t.SenderAddress.Equal(&addr) {
				return true
			}
		}
	}

	return false
}

// resolveBlockRange returns the start and latest headers based on the blockID.
// It will also do some sanity checks and return errors if the blockID is invalid.
func (h *Handler) resolveBlockRange(
	blockID *SubscriptionBlockID,
) (*core.Header, *core.Header, *jsonrpc.Error) {
	latestHeader, err := h.bcReader.HeadsHeader()
	if err != nil {
		return nil, nil, rpccore.ErrInternal.CloneWithData(err.Error())
	}

	if blockID == nil || blockID.IsLatest() {
		return latestHeader, latestHeader, nil
	}

	startHeader, rpcErr := h.blockHeaderByID((*BlockID)(blockID))
	if rpcErr != nil {
		return nil, nil, rpcErr
	}

	if latestHeader.Number >= rpccore.MaxBlocksBack &&
		startHeader.Number <= latestHeader.Number-rpccore.MaxBlocksBack {
		return nil, nil, rpccore.ErrTooManyBlocksBack
	}

	return startHeader, latestHeader, nil
}

type ReorgEvent struct {
	StartBlockHash *felt.Felt `json:"starting_block_hash"`
	StartBlockNum  uint64     `json:"starting_block_number"`
	EndBlockHash   *felt.Felt `json:"ending_block_hash"`
	EndBlockNum    uint64     `json:"ending_block_number"`
}

func (h *Handler) Unsubscribe(ctx context.Context, id string) (bool, *jsonrpc.Error) {
	w, ok := jsonrpc.ConnFromContext(ctx)
	if !ok {
		return false, jsonrpc.Err(jsonrpc.MethodNotFound, nil)
	}
	sub, ok := h.subscriptions.Load(id)
	if !ok {
		return false, rpccore.ErrInvalidSubscriptionID
	}

	subs := sub.(*subscription)
	if !subs.conn.Equal(w) {
		return false, rpccore.ErrInvalidSubscriptionID
	}

	subs.cancel()
	subs.wg.Wait() // Let the subscription finish before responding.
	h.subscriptions.Delete(id)
	return true, nil
}

type TxnFinalityStatusWithoutL1 TxnFinalityStatus

func (s *TxnFinalityStatusWithoutL1) UnmarshalText(text []byte) error {
	var base TxnFinalityStatus
	if err := base.UnmarshalText(text); err != nil {
		return err
	}
	// Validate that only non-L1 statuses are allowed
	if base == TxnAcceptedOnL1 {
		return fmt.Errorf("invalid TxnStatus: %s;", text)
	}
	*s = TxnFinalityStatusWithoutL1(base)
	return nil
}

func (s TxnFinalityStatusWithoutL1) MarshalText() ([]byte, error) {
	switch s {
	case TxnFinalityStatusWithoutL1(TxnPreConfirmed):
		return []byte("PRE_CONFIRMED"), nil
	case TxnFinalityStatusWithoutL1(TxnAcceptedOnL2):
		return []byte("ACCEPTED_ON_L2"), nil
	default:
		return nil, fmt.Errorf("unknown TxnFinalityStatusWithoutL1 %v", s)
	}
}

type TxnStatusWithoutL1 TxnStatus

func (s *TxnStatusWithoutL1) UnmarshalText(text []byte) error {
	switch string(text) {
	case "RECEIVED":
		*s = TxnStatusWithoutL1(TxnStatusReceived)
		return nil
	case "CANDIDATE":
		*s = TxnStatusWithoutL1(TxnStatusCandidate)
		return nil
	case "PRE_CONFIRMED":
		*s = TxnStatusWithoutL1(TxnStatusPreConfirmed)
		return nil
	case "ACCEPTED_ON_L2":
		*s = TxnStatusWithoutL1(TxnStatusAcceptedOnL2)
		return nil
	default:
		return fmt.Errorf("invalid TxnStatus: %s;", text)
	}
}

func (s TxnStatusWithoutL1) MarshalText() ([]byte, error) {
	switch s {
	case TxnStatusWithoutL1(TxnStatusReceived):
		return []byte("RECEIVED"), nil
	case TxnStatusWithoutL1(TxnStatusCandidate):
		return []byte("CANDIDATE"), nil
	case TxnStatusWithoutL1(TxnStatusPreConfirmed):
		return []byte("PRE_CONFIRMED"), nil
	case TxnStatusWithoutL1(TxnStatusAcceptedOnL2):
		return []byte("ACCEPTED_ON_L2"), nil
	default:
		return nil, fmt.Errorf("unknown TxnFinalityStatusWithoutL1 %v", s)
	}
}

func sendReorg(w jsonrpc.Conn, reorg *sync.ReorgBlockRange, id string) error {
	return sendResponse("starknet_subscriptionReorg", w, id, &ReorgEvent{
		StartBlockHash: reorg.StartBlockHash,
		StartBlockNum:  reorg.StartBlockNum,
		EndBlockHash:   reorg.EndBlockHash,
		EndBlockNum:    reorg.EndBlockNum,
	})
}

func sendResponse(method string, w jsonrpc.Conn, id string, result any) error {
	resp, err := json.Marshal(SubscriptionResponse{
		Version: "2.0",
		Method:  method,
		Params: map[string]any{
			"subscription_id": id,
			"result":          result,
		},
	})
	if err != nil {
		return err
	}
	_, err = w.Write(resp)
	return err
}
