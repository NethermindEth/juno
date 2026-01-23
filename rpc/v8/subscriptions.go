package rpcv8

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/sync"
)

const subscribeEventsChunkSize = 1024

// The function signature of SubscribeTransactionStatus cannot be changed since the jsonrpc package maps the number
// of argument in the function to the parameters in the starknet spec, therefore, the following variables are not passed
// as arguments, and they can be modified in the test to make them run faster.
var (
	subscribeTxStatusTimeout        = 5 * time.Minute
	subscribeTxStatusTickerDuration = 5 * time.Second
)

type SubscriptionResponse struct {
	Version string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type errorTxnHashNotFound struct {
	txHash felt.Felt
}

func (e errorTxnHashNotFound) Error() string {
	return fmt.Sprintf("transaction %v not found", e.txHash)
}

// As per the spec, this is the same as BlockID, but without `pending`
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

	if blockID.IsPending() {
		return errors.New("subscription block id cannot be pending")
	}

	return nil
}

type on[T any] func(ctx context.Context, id string, sub *subscription, event T) error

type subscriber struct {
	onStart       on[any]
	onReorg       on[*sync.ReorgBlockRange]
	onNewHead     on[*core.Block]
	onPendingData on[core.PendingData]
	onL1Head      on[*core.L1Head]
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

	sub.wg.Go(func() {
		defer func() {
			h.unsubscribe(sub, id)
			unsubscribeFeedSubscription(reorgSub)
			unsubscribeFeedSubscription(l1HeadSub)
			unsubscribeFeedSubscription(newHeadsSub)
			unsubscribeFeedSubscription(pendingDataSub)
		}()

		if subscriber.onStart != nil {
			if err := subscriber.onStart(subscriptionCtx, id, sub, nil); err != nil {
				h.log.Warnw("Error starting subscription", "err", err)
				return
			}
		}

		for {
			select {
			case <-subscriptionCtx.Done():
				return
			case reorg := <-reorgRecv:
				if err := subscriber.onReorg(subscriptionCtx, id, sub, reorg); err != nil {
					h.log.Warnw("Error on reorg", "id", id, "err", err)
					return
				}
			case l1Head := <-l1HeadRecv:
				if err := subscriber.onL1Head(subscriptionCtx, id, sub, l1Head); err != nil {
					h.log.Warnw("Error on l1 head", "id", id, "err", err)
					return
				}
			case head := <-newHeadsRecv:
				if err := subscriber.onNewHead(subscriptionCtx, id, sub, head); err != nil {
					h.log.Warnw("Error on new head", "id", id, "err", err)
					return
				}
			case pending := <-pendingRecv:
				if err := subscriber.onPendingData(subscriptionCtx, id, sub, pending); err != nil {
					h.log.Warnw("Error on pending", "id", id, "err", err)
					return
				}
			}
		}
	})

	return SubscriptionID(id), nil
}

// Currently the order of transactions is deterministic, so the transaction always execute on a deterministic state
// Therefore, the emitted events are deterministic and we can use the transaction hash and event index to identify.
type SentEvent struct {
	TransactionHash felt.Felt
	EventIndex      uint
}

// SubscribeEvents creates a WebSocket stream which will fire events for new Starknet events with applied filters
func (h *Handler) SubscribeEvents(
	ctx context.Context, fromAddr *felt.Felt, keys [][]felt.Felt, blockID *SubscriptionBlockID,
) (SubscriptionID, *jsonrpc.Error) {
	w, ok := jsonrpc.ConnFromContext(ctx)
	if !ok {
		return "", jsonrpc.Err(jsonrpc.MethodNotFound, nil)
	}

	lenKeys := len(keys)
	for _, k := range keys {
		lenKeys += len(k)
	}
	if lenKeys > rpccore.MaxEventFilterKeys {
		return "", rpccore.ErrTooManyKeysInFilter
	}

	requestedHeader, headHeader, rpcErr := h.resolveBlockRange(blockID)
	if rpcErr != nil {
		return "", rpcErr
	}

	nextBlock := headHeader.Number + 1
	eventsPreviouslySent := make(map[SentEvent]struct{})
	pendingID := BlockIDPending()
	subscriber := subscriber{
		onStart: func(ctx context.Context, id string, _ *subscription, _ any) error {
			fromB := BlockIDFromNumber(requestedHeader.Number)
			toB := BlockIDFromNumber(headHeader.Number)
			return h.processEvents(
				ctx, w, id, &fromB, &toB, fromAddr, keys, nil, headHeader.Number,
			)
		},
		onReorg: func(
			ctx context.Context, id string, _ *subscription, reorg *sync.ReorgBlockRange,
		) error {
			if err := sendReorg(w, reorg, id); err != nil {
				return err
			}
			nextBlock = reorg.StartBlockNum
			return nil
		},
		onNewHead: func(ctx context.Context, id string, _ *subscription, head *core.Block) error {
			fromB := BlockIDFromNumber(nextBlock)
			toB := BlockIDFromNumber(head.Number)
			err := h.processEvents(
				ctx, w, id, &fromB, &toB, fromAddr, keys, eventsPreviouslySent, head.Number,
			)
			if err != nil {
				return err
			}
			nextBlock = head.Number + 1
			return nil
		},
		onPendingData: func(ctx context.Context, id string, _ *subscription, pending core.PendingData) error {
			if pending == nil || pending.Variant() != core.PendingBlockVariant {
				return nil
			}

			return h.processEvents(
				ctx,
				w,
				id,
				&pendingID,
				&pendingID,
				fromAddr,
				keys,
				eventsPreviouslySent,
				pending.GetBlock().Number-1,
			)
		},
	}
	return h.subscribe(ctx, w, subscriber)
}

// SubscribeTransactionStatus subscribes to status changes of a transaction. It checks for updates each time a new block is added.
// Later updates are sent only when the transaction status changes.
// The optional block_id parameter is ignored, as status changes are not stored and historical data cannot be sent.
func (h *Handler) SubscribeTransactionStatus(ctx context.Context, txHash *felt.Felt) (SubscriptionID, *jsonrpc.Error) {
	w, ok := jsonrpc.ConnFromContext(ctx)
	if !ok {
		return "", jsonrpc.Err(jsonrpc.MethodNotFound, nil)
	}

	var lastStatus TxnStatus
	var err error

	subscriber := subscriber{
		onStart: func(ctx context.Context, id string, sub *subscription, _ any) error {
			if lastStatus, err = h.getInitialTxStatus(ctx, sub, id, txHash); err != nil {
				return err
			}
			return nil
		},
		onReorg: func(ctx context.Context, id string, _ *subscription, reorg *sync.ReorgBlockRange) error {
			return sendReorg(w, reorg, id)
		},
		onNewHead: func(ctx context.Context, id string, sub *subscription, head *core.Block) error {
			if lastStatus < TxnStatusAcceptedOnL2 {
				if lastStatus, err = h.checkTxStatus(ctx, sub, id, txHash, lastStatus); err != nil {
					return err
				}
			}
			return nil
		},
		onPendingData: func(ctx context.Context, id string, sub *subscription, pending core.PendingData) error {
			if pending == nil || pending.Variant() != core.PendingBlockVariant {
				return nil
			}

			if lastStatus < TxnStatusAcceptedOnL2 {
				if lastStatus, err = h.checkTxStatus(ctx, sub, id, txHash, lastStatus); err != nil {
					return err
				}
			}
			return nil
		},
		onL1Head: func(ctx context.Context, id string, sub *subscription, l1Head *core.L1Head) error {
			if lastStatus, err = h.checkTxStatus(ctx, sub, id, txHash, lastStatus); err != nil {
				return err
			}
			return nil
		},
	}
	return h.subscribe(ctx, w, subscriber)
}

// If the error is transaction not found that means the transaction has not been submitted to the feeder gateway,
// therefore, we need to wait for a specified time and at regular interval check if the transaction has been found.
// If the transaction is found during the timout expiry, then we continue to keep track of its status otherwise the
// websocket connection is closed after the expiry.
func (h *Handler) getInitialTxStatus(ctx context.Context, sub *subscription, id string, txHash *felt.Felt) (TxnStatus, error) {
	var lastStatus TxnStatus
	var err error
	if lastStatus, err = h.checkTxStatus(ctx, sub, id, txHash, 0); !errors.Is(err, errorTxnHashNotFound{*txHash}) {
		return lastStatus, err
	}

	ctx, cancelTimeout := context.WithTimeout(ctx, subscribeTxStatusTimeout)
	defer cancelTimeout()
	ticker := time.Tick(subscribeTxStatusTickerDuration)

	for {
		select {
		case <-ctx.Done():
			return lastStatus, err
		case <-ticker:
			if lastStatus, err = h.checkTxStatus(ctx, sub, id, txHash, lastStatus); !errors.Is(err, errorTxnHashNotFound{*txHash}) {
				return lastStatus, err
			}
		}
	}
}

func (h *Handler) checkTxStatus(
	ctx context.Context,
	sub *subscription,
	id string,
	txHash *felt.Felt,
	lastStatus TxnStatus,
) (TxnStatus, error) {
	status, rpcErr := h.TransactionStatus(ctx, *txHash)
	if rpcErr != nil {
		if rpcErr != rpccore.ErrTxnHashNotFound {
			return lastStatus, fmt.Errorf("error while checking status for transaction %v with rpc error message: %v", txHash, rpcErr.Message)
		}
		return lastStatus, errorTxnHashNotFound{*txHash}
	}

	if status.Finality == lastStatus {
		return lastStatus, nil
	}

	if err := sendTxnStatus(sub.conn, SubscriptionTransactionStatus{txHash, *status}, id); err != nil {
		return lastStatus, err
	}

	if status.Finality == TxnStatusRejected || status.Finality == TxnStatusAcceptedOnL1 {
		sub.cancel()
	}

	return status.Finality, nil
}

// processEvents queries database for events and stream filtered events.
func (h *Handler) processEvents(
	ctx context.Context,
	w jsonrpc.Conn,
	id string,
	from, to *BlockID,
	fromAddr *felt.Felt,
	keys [][]felt.Felt,
	eventsPreviouslySent map[SentEvent]struct{},
	height uint64,
) error {
	filter, err := h.bcReader.EventFilter([]*felt.Felt{fromAddr}, keys, h.PendingData)
	if err != nil {
		return err
	}

	defer h.callAndLogErr(filter.Close, "error closing event filter in events subscription")

	err = setEventFilterRange(filter, from, to, height)
	if err != nil {
		return err
	}

	filteredEvents, cToken, err := filter.Events(nil, subscribeEventsChunkSize)
	if err != nil {
		return err
	}

	err = sendEvents(ctx, w, filteredEvents, eventsPreviouslySent, id)
	if err != nil {
		return err
	}

	for !cToken.IsEmpty() {
		filteredEvents, cToken, err = filter.Events(&cToken, subscribeEventsChunkSize)
		if err != nil {
			return err
		}

		err = sendEvents(ctx, w, filteredEvents, eventsPreviouslySent, id)
		if err != nil {
			return err
		}
	}
	return nil
}

func sendEvents(
	ctx context.Context,
	w jsonrpc.Conn,
	events []blockchain.FilteredEvent,
	eventsPreviouslySent map[SentEvent]struct{},
	id string,
) error {
	for _, event := range events {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if eventsPreviouslySent != nil {
				sentEvent := SentEvent{
					TransactionHash: *event.TransactionHash,
					EventIndex:      event.EventIndex,
				}
				if _, ok := eventsPreviouslySent[sentEvent]; ok {
					continue
				}
				// This describe the lifecycle of SentEvent.
				// It's added when the event is received from a pending block.
				// It's deleted when the event is received from a head block.
				if isPending := event.BlockHash == nil; isPending {
					eventsPreviouslySent[sentEvent] = struct{}{}
				} else {
					delete(eventsPreviouslySent, sentEvent)
				}
			}

			emittedEvent := &EmittedEvent{
				BlockNumber:     event.BlockNumber, // This always be filled as subscribeEvents cannot be called on pending block
				BlockHash:       event.BlockHash,
				TransactionHash: event.TransactionHash,
				Event: &Event{
					From: event.From,
					Keys: event.Keys,
					Data: event.Data,
				},
			}

			if err := sendResponse("starknet_subscriptionEvents", w, id, emittedEvent); err != nil {
				return err
			}
		}
	}
	return nil
}

// SubscribeNewHeads creates a WebSocket stream which will fire events when a new block header is added.
func (h *Handler) SubscribeNewHeads(ctx context.Context, blockID *SubscriptionBlockID) (SubscriptionID, *jsonrpc.Error) {
	w, ok := jsonrpc.ConnFromContext(ctx)
	if !ok {
		return "", jsonrpc.Err(jsonrpc.MethodNotFound, nil)
	}

	startHeader, latestHeader, rpcErr := h.resolveBlockRange(blockID)
	if rpcErr != nil {
		return "", rpcErr
	}

	subscriber := subscriber{
		onStart: func(ctx context.Context, id string, _ *subscription, _ any) error {
			return h.sendHistoricalHeaders(ctx, startHeader, latestHeader, w, id)
		},
		onReorg: func(ctx context.Context, id string, _ *subscription, reorg *sync.ReorgBlockRange) error {
			return sendReorg(w, reorg, id)
		},
		onNewHead: func(ctx context.Context, id string, _ *subscription, head *core.Block) error {
			return sendHeader(w, head.Header, id)
		},
	}
	return h.subscribe(ctx, w, subscriber)
}

// SubscribePendingTxs creates a WebSocket stream which will fire events when a new pending transaction is added.
// The getDetails flag controls if the response will contain the transaction details or just the transaction hashes.
// The senderAddr flag is used to filter the transactions by sender address.
func (h *Handler) SubscribePendingTxs(ctx context.Context, getDetails *bool, senderAddr []felt.Felt) (SubscriptionID, *jsonrpc.Error) {
	w, ok := jsonrpc.ConnFromContext(ctx)
	if !ok {
		return "", jsonrpc.Err(jsonrpc.MethodNotFound, nil)
	}

	if len(senderAddr) > rpccore.MaxEventFilterKeys {
		return "", rpccore.ErrTooManyAddressesInFilter
	}

	sentTxHashes := make(map[felt.Felt]struct{})
	lastParentHash := felt.Zero

	subscriber := subscriber{
		onStart: func(ctx context.Context, id string, _ *subscription, _ any) error {
			if pending := h.PendingBlock(); pending != nil {
				return h.onPendingBlock(id, w, getDetails, senderAddr, pending, &lastParentHash, sentTxHashes)
			}
			return nil
		},
		onPendingData: func(ctx context.Context, id string, _ *subscription, pending core.PendingData) error {
			if pending == nil || pending.Variant() != core.PendingBlockVariant {
				return nil
			}

			return h.onPendingBlock(id, w, getDetails, senderAddr, pending.GetBlock(), &lastParentHash, sentTxHashes)
		},
	}
	return h.subscribe(ctx, w, subscriber)
}

// If getDetails is true, response will contain the transaction details.
// If getDetails is false, response will only contain the transaction hashes.
func (h *Handler) onPendingBlock(
	id string,
	w jsonrpc.Conn,
	getDetails *bool,
	senderAddr []felt.Felt,
	pending *core.Block,
	lastParentHash *felt.Felt,
	sentTxHashes map[felt.Felt]struct{},
) error {
	if !pending.ParentHash.Equal(lastParentHash) {
		clear(sentTxHashes)
		*lastParentHash = *pending.ParentHash
	}

	var toResult func(txn core.Transaction) any
	if getDetails != nil && *getDetails {
		toResult = toFullTx
	} else {
		toResult = toHash
	}

	for _, txn := range pending.Transactions {
		if _, exist := sentTxHashes[*txn.Hash()]; !exist {
			if h.filterTxBySender(txn, senderAddr) {
				if err := sendPendingTxs(w, toResult(txn), id); err != nil {
					return err
				}
			}
			sentTxHashes[*txn.Hash()] = struct{}{}
		}
	}
	return nil
}

func toFullTx(txn core.Transaction) any {
	return AdaptTransaction(txn)
}

func toHash(txn core.Transaction) any {
	return txn.Hash()
}

// filterTxBySender checks if the transaction is included in the sender address list.
// If the sender address list is empty, it will return true by default.
// If the sender address list is not empty, it will check if the transaction is an Invoke or Declare transaction
// and if the sender address is in the list. For other transaction types, it will by default return false.
func (h *Handler) filterTxBySender(txn core.Transaction, senderAddr []felt.Felt) bool {
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

func sendPendingTxs(w jsonrpc.Conn, result any, id string) error {
	return sendResponse("starknet_subscriptionPendingTransactions", w, id, result)
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

// sendHistoricalHeaders sends a range of headers from the start header until the latest header
func (h *Handler) sendHistoricalHeaders(
	ctx context.Context,
	startHeader, latestHeader *core.Header,
	w jsonrpc.Conn,
	id string,
) error {
	var (
		err       error
		curHeader = startHeader
	)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := sendHeader(w, curHeader, id); err != nil {
				return err
			}

			if curHeader.Number == latestHeader.Number {
				return nil
			}

			curHeader, err = h.bcReader.BlockHeaderByNumber(curHeader.Number + 1)
			if err != nil {
				return err
			}
		}
	}
}

// sendHeader creates a request and sends it to the client
func sendHeader(w jsonrpc.Conn, header *core.Header, id string) error {
	return sendResponse("starknet_subscriptionNewHeads", w, id, adaptBlockHeader(header))
}

type ReorgEvent struct {
	StartBlockHash *felt.Felt `json:"starting_block_hash"`
	StartBlockNum  uint64     `json:"starting_block_number"`
	EndBlockHash   *felt.Felt `json:"ending_block_hash"`
	EndBlockNum    uint64     `json:"ending_block_number"`
}

func sendReorg(w jsonrpc.Conn, reorg *sync.ReorgBlockRange, id string) error {
	return sendResponse("starknet_subscriptionReorg", w, id, &ReorgEvent{
		StartBlockHash: reorg.StartBlockHash,
		StartBlockNum:  reorg.StartBlockNum,
		EndBlockHash:   reorg.EndBlockHash,
		EndBlockNum:    reorg.EndBlockNum,
	})
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

type SubscriptionTransactionStatus struct {
	TransactionHash *felt.Felt        `json:"transaction_hash"`
	Status          TransactionStatus `json:"status"`
}

// sendTxnStatus creates a response and sends it to the client
func sendTxnStatus(w jsonrpc.Conn, status SubscriptionTransactionStatus, id string) error {
	return sendResponse("starknet_subscriptionTransactionStatus", w, id, status)
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
