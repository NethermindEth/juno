package rpcv9

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
)

const subscribeEventsChunkSize = 1024

// The function signature of SubscribeTransactionStatus cannot be changed since the jsonrpc package maps the number
// of argument in the function to the parameters in the starknet spec, therefore, the following variables are not passed
// as arguments, and they can be modified in the test to make them run faster.
var (
	subscribeTxStatusTimeout        = 5 * time.Minute
	subscribeTxStatusTickerDuration = time.Second
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
					h.log.Warnw("Error on pending data", "id", id, "err", err)
					return
				}
			case preLatest := <-preLatestRecv:
				if err := subscriber.onPreLatest(subscriptionCtx, id, sub, preLatest); err != nil {
					h.log.Warnw("Error on  preLatest", "id", id, "err", err)
					return
				}
			}
		}
	})

	return SubscriptionID(id), nil
}

type SubscriptionEmittedEvent struct {
	rpcv6.EmittedEvent
	FinalityStatus TxnFinalityStatus `json:"finality_status"`
}

// Currently the order of transactions is deterministic, so the transaction always execute on a deterministic state
// Therefore, the emitted events are deterministic and we can use the transaction hash and event index to identify.
type SentEvent struct {
	TransactionHash felt.Felt
	EventIndex      uint
}

// SubscribeEvents creates a WebSocket stream which will fire events for new Starknet events with applied filters
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/c2e93098b9c2ca0423b7f4d15b201f52f22d8c36/api/starknet_ws_api.json#L59
func (h *Handler) SubscribeEvents(
	ctx context.Context,
	fromAddr *felt.Felt,
	keys [][]felt.Felt,
	blockID *SubscriptionBlockID,
	finalityStatus *TxnFinalityStatusWithoutL1,
) (SubscriptionID, *jsonrpc.Error) {
	w, ok := jsonrpc.ConnFromContext(ctx)
	if !ok {
		return "", jsonrpc.Err(jsonrpc.MethodNotFound, nil)
	}

	lenKeys := len(keys)
	for _, k := range keys {
		if lenKeys += len(k); lenKeys > rpccore.MaxEventFilterKeys {
			return "", rpccore.ErrTooManyKeysInFilter
		}
	}

	requestedHeader, headHeader, rpcErr := h.resolveBlockRange(blockID)
	if rpcErr != nil {
		return "", rpcErr
	}
	// default to ACCEPTED_ON_L2
	if finalityStatus == nil {
		finalityStatus = utils.HeapPtr(TxnFinalityStatusWithoutL1(TxnAcceptedOnL2))
	}

	l1Head, err := h.bcReader.L1Head()
	if err != nil {
		return "", rpccore.ErrInternal.CloneWithData(err.Error())
	}

	l1HeadNumber := l1Head.BlockNumber
	sentCache := rpccore.NewSubscriptionCache[SentEvent, TxnFinalityStatus]()
	eventMatcher := blockchain.NewEventMatcher(fromAddr, keys)
	subscriber := subscriber{
		onStart: func(ctx context.Context, id string, _ *subscription, _ any) error {
			fromBlock := BlockIDFromNumber(requestedHeader.Number)
			var toBlock BlockID
			if *finalityStatus == TxnFinalityStatusWithoutL1(TxnPreConfirmed) {
				toBlock = BlockIDPreConfirmed()
			} else {
				toBlock = BlockIDFromNumber(headHeader.Number)
			}

			return h.processHistoricalEvents(
				ctx,
				w,
				id,
				&fromBlock,
				&toBlock,
				fromAddr,
				keys,
				sentCache,
				headHeader.Number,
				l1HeadNumber,
			)
		},
		onReorg: func(ctx context.Context, id string, _ *subscription, reorg *sync.ReorgBlockRange) error {
			sentCache.Clear()
			return sendReorg(w, reorg, id)
		},
		onNewHead: func(ctx context.Context, id string, _ *subscription, head *core.Block) error {
			return processBlockEvents(
				ctx,
				w,
				id,
				head,
				fromAddr,
				&eventMatcher,
				sentCache,
				TxnAcceptedOnL2,
				false,
			)
		},
		onPreLatest: func(
			ctx context.Context,
			id string,
			_ *subscription,
			preLatest *core.PreLatest,
		) error {
			return processBlockEvents(
				ctx,
				w,
				id,
				preLatest.Block,
				fromAddr,
				&eventMatcher,
				sentCache,
				TxnAcceptedOnL2,
				true,
			)
		},
		onPendingData: func(ctx context.Context, id string, _ *subscription, pending core.PendingData) error {
			var blockFinalityStatus TxnFinalityStatus
			switch v := pending.Variant(); v {
			case core.PendingBlockVariant:
				blockFinalityStatus = TxnAcceptedOnL2
			case core.PreConfirmedBlockVariant:
				if *finalityStatus != TxnFinalityStatusWithoutL1(TxnPreConfirmed) {
					return nil
				}
				blockFinalityStatus = TxnPreConfirmed
			default:
				return fmt.Errorf("unknown pending variant %v", v)
			}

			return processBlockEvents(
				ctx,
				w,
				id,
				pending.GetBlock(),
				fromAddr,
				&eventMatcher,
				sentCache,
				blockFinalityStatus,
				false,
			)
		},
	}
	return h.subscribe(ctx, w, subscriber)
}

// processHistoricalEvents queries database for events and stream filtered events.
func (h *Handler) processHistoricalEvents(
	ctx context.Context,
	w jsonrpc.Conn,
	id string,
	from, to *BlockID,
	fromAddr *felt.Felt,
	keys [][]felt.Felt,
	sentCache *rpccore.SubscriptionCache[SentEvent, TxnFinalityStatus],
	height uint64,
	l1Head uint64,
) error {
	filter, err := h.bcReader.EventFilter(fromAddr, keys, h.PendingData)
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

	err = sendEvents(ctx, w, filteredEvents, sentCache, id, height, l1Head)
	if err != nil {
		return err
	}

	for !cToken.IsEmpty() {
		filteredEvents, cToken, err = filter.Events(&cToken, subscribeEventsChunkSize)
		if err != nil {
			return err
		}

		err = sendEvents(ctx, w, filteredEvents, sentCache, id, height, l1Head)
		if err != nil {
			return err
		}
	}
	return nil
}

// processBlockEvents, extract events from block and stream filtered events.
func processBlockEvents(
	ctx context.Context,
	w jsonrpc.Conn,
	id string,
	block *core.Block,
	fromAddr *felt.Felt,
	eventMatcher *blockchain.EventMatcher,
	sentCache *rpccore.SubscriptionCache[SentEvent, TxnFinalityStatus],
	finalityStatus TxnFinalityStatus,
	isPreLatest bool,
) error {
	if isMatch := eventMatcher.TestBloom(block.EventsBloom); !isMatch {
		return nil
	}

	var blockNumber *uint64
	// if header.Hash == nil and parentHash != nil it's a pending block
	// if header.Hash == nil and parentHash == nil it's a pre_confirmed block
	isNotPending := block.Hash != nil || block.ParentHash == nil || isPreLatest
	if isNotPending {
		blockNumber = &block.Number
	}

	for _, receipt := range block.Receipts {
		for i, event := range receipt.Events {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if fromAddr != nil && !event.From.Equal(fromAddr) {
				continue
			}

			if !eventMatcher.MatchesEventKeys(event.Keys) {
				continue
			}

			event := blockchain.FilteredEvent{
				BlockNumber:     blockNumber,
				BlockHash:       block.Hash,
				TransactionHash: receipt.TransactionHash,
				EventIndex:      uint(i),
				Event:           event,
			}

			err := sendEventWithoutDuplicate(
				w,
				&event,
				sentCache,
				id,
				finalityStatus,
				&block.Number,
			)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// sendEvents streams filtered events, does not stream last sent status multiple times
func sendEvents(
	ctx context.Context,
	w jsonrpc.Conn,
	events []blockchain.FilteredEvent,
	sentCache *rpccore.SubscriptionCache[SentEvent, TxnFinalityStatus],
	id string,
	height uint64,
	l1Head uint64,
) error {
	for _, event := range events {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			var finalityStatus TxnFinalityStatus
			switch {
			case event.BlockNumber == nil: // pending block
				finalityStatus = TxnAcceptedOnL2
			case *event.BlockNumber > height: // pre_confirmed or pre_latest block
				if event.BlockParentHash == nil {
					finalityStatus = TxnPreConfirmed
				} else {
					finalityStatus = TxnAcceptedOnL2
				}
			case *event.BlockNumber <= l1Head:
				finalityStatus = TxnAcceptedOnL1
			default: // Canonical block not finalised on L1
				finalityStatus = TxnAcceptedOnL2
			}

			if err := sendEventWithoutDuplicate(
				w,
				&event,
				sentCache,
				id,
				finalityStatus,
				event.BlockNumber,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

// sendEventWithoutDuplicate streams event if status is changed
func sendEventWithoutDuplicate(
	w jsonrpc.Conn,
	event *blockchain.FilteredEvent,
	sentCache *rpccore.SubscriptionCache[SentEvent, TxnFinalityStatus],
	id string,
	finalityStatus TxnFinalityStatus,
	blockNum *uint64,
) error {
	// TODO: Remove this check when we drop support for starknet < 0.14.0.
	// Only use cache for deduplication if we have a block number
	if blockNum != nil {
		sentEvent := SentEvent{
			TransactionHash: *event.TransactionHash,
			EventIndex:      event.EventIndex,
		}
		if !sentCache.ShouldSend(*blockNum, &sentEvent, &finalityStatus) {
			return nil
		}
		sentCache.Put(*blockNum, &sentEvent, &finalityStatus)
	}

	emittedEvent := rpcv6.EmittedEvent{
		BlockNumber:     event.BlockNumber,
		BlockHash:       event.BlockHash,
		TransactionHash: event.TransactionHash,
		Event: &rpcv6.Event{
			From: event.From,
			Keys: event.Keys,
			Data: event.Data,
		},
	}

	response := &SubscriptionEmittedEvent{
		EmittedEvent:   emittedEvent,
		FinalityStatus: finalityStatus,
	}

	return sendEvent(w, response, id)
}

type SubscriptionTransactionStatus struct {
	TransactionHash *felt.Felt        `json:"transaction_hash"`
	Status          TransactionStatus `json:"status"`
}

// SubscribeTransactionStatus subscribes to status changes of a transaction. It checks for updates each time a new block is added.
// Later updates are sent only when the transaction status changes.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/c2e93098b9c2ca0423b7f4d15b201f52f22d8c36/api/starknet_ws_api.json#L151
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
			lastStatus, err = h.checkTxStatusIfPending(ctx, sub, id, txHash, lastStatus)
			return err
		},
		onPreLatest: func(
			ctx context.Context,
			id string,
			sub *subscription,
			preLatest *core.PreLatest,
		) error {
			lastStatus, err = h.checkTxStatusIfPending(ctx, sub, id, txHash, lastStatus)
			return err
		},
		onPendingData: func(ctx context.Context, id string, sub *subscription, pending core.PendingData) error {
			lastStatus, err = h.checkTxStatusIfPending(ctx, sub, id, txHash, lastStatus)
			return err
		},
		onL1Head: func(ctx context.Context, id string, sub *subscription, l1Head *core.L1Head) error {
			lastStatus, err = h.checkTxStatus(ctx, sub, id, txHash, lastStatus)
			return err
		},
	}
	return h.subscribe(ctx, w, subscriber)
}

// If the error is transaction not found that means the transaction has not been submitted to the feeder gateway,
// therefore, we need to wait for a specified time and at regular interval check if the transaction has been found.
// If the transaction is found during the timeout expiry, then we continue to keep track of its status otherwise the
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

// checkTxStatusIfPending checks the transaction status only if the last known status
// is less than TxnStatusAcceptedOnL2 (i.e., the transaction is still pending).
// If the transaction has already reached or surpassed TxnStatusAcceptedOnL2,
// it returns the last known status without making an additional status check.
func (h *Handler) checkTxStatusIfPending(
	ctx context.Context,
	sub *subscription,
	id string,
	txHash *felt.Felt,
	lastStatus TxnStatus,
) (TxnStatus, error) {
	if lastStatus < TxnStatusAcceptedOnL2 {
		return h.checkTxStatus(ctx, sub, id, txHash, lastStatus)
	}
	return lastStatus, nil
}

func (h *Handler) checkTxStatus(
	ctx context.Context,
	sub *subscription,
	id string,
	txHash *felt.Felt,
	lastStatus TxnStatus,
) (TxnStatus, error) {
	status, rpcErr := h.TransactionStatus(ctx, txHash)
	if rpcErr != nil {
		if rpcErr != rpccore.ErrTxnHashNotFound {
			return lastStatus, fmt.Errorf("error while checking status for transaction %v with rpc error message: %v", txHash, rpcErr.Message)
		}
		return lastStatus, errorTxnHashNotFound{*txHash}
	}

	if status.Finality == lastStatus {
		return lastStatus, nil
	}

	if err := sendTxnStatus(sub.conn, SubscriptionTransactionStatus{txHash, status}, id); err != nil {
		return lastStatus, err
	}

	if status.Finality == TxnStatusAcceptedOnL1 {
		sub.cancel()
	}

	return status.Finality, nil
}

// SubscribeNewHeads creates a WebSocket stream which will fire events when a new block header is added.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/c2e93098b9c2ca0423b7f4d15b201f52f22d8c36/api/starknet_ws_api.json#L10
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

// filterTxBySender checks if the transaction is included in the sender address list.
// If the sender address list is empty, it will return true by default.
// If the sender address list is not empty, it will check if the transaction is an Invoke or Declare transaction
// and if the sender address is in the list. For other transaction types, it will by default return false.
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

type SentReceipt struct {
	TransactionHash  felt.Felt
	TransactionIndex int
}

// SubscribeTransactionReceipts creates a WebSocket stream which will fire events when new transaction receipts are created.
// The endpoint receives a vector of finality statuses. An event is fired for each finality status update.
// It is possible for receipts for pre-confirmed transactions to be received multiple times, or not at all.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/4e98e3684b50ee9e63b7eeea9412b6a2ed7494ec/api/starknet_ws_api.json#L186
func (h *Handler) SubscribeNewTransactionReceipts(
	ctx context.Context,
	senderAddress []felt.Felt,
	finalityStatuses []TxnFinalityStatusWithoutL1,
) (SubscriptionID, *jsonrpc.Error) {
	w, ok := jsonrpc.ConnFromContext(ctx)
	if !ok {
		return "", jsonrpc.Err(jsonrpc.MethodNotFound, nil)
	}

	if len(senderAddress) > rpccore.MaxEventFilterKeys {
		return "", rpccore.ErrTooManyAddressesInFilter
	}

	if len(finalityStatuses) == 0 {
		finalityStatuses = []TxnFinalityStatusWithoutL1{TxnFinalityStatusWithoutL1(TxnAcceptedOnL2)}
	} else {
		finalityStatuses = utils.Set(finalityStatuses)
	}

	sentCache := rpccore.NewSubscriptionCache[SentReceipt, TxnFinalityStatusWithoutL1]()

	subscriber := subscriber{
		onNewHead: func(ctx context.Context, id string, _ *subscription, head *core.Block) error {
			if !slices.Contains(finalityStatuses, TxnFinalityStatusWithoutL1(TxnAcceptedOnL2)) {
				return nil
			}

			return processBlockReceipts(
				id,
				w,
				senderAddress,
				head,
				sentCache,
				TxnFinalityStatusWithoutL1(TxnAcceptedOnL2),
				false,
			)
		},
		onPreLatest: func(
			ctx context.Context,
			id string,
			_ *subscription,
			preLatest *core.PreLatest,
		) error {
			if !slices.Contains(finalityStatuses, TxnFinalityStatusWithoutL1(TxnAcceptedOnL2)) {
				return nil
			}

			return processBlockReceipts(
				id,
				w,
				senderAddress,
				preLatest.Block,
				sentCache,
				TxnFinalityStatusWithoutL1(TxnAcceptedOnL2),
				true,
			)
		},
		onPendingData: func(ctx context.Context, id string, sub *subscription, pending core.PendingData) error {
			if pending == nil {
				return nil
			}
			block := pending.GetBlock()
			var blockFinalityStatus TxnFinalityStatusWithoutL1
			switch v := pending.Variant(); v {
			case core.PendingBlockVariant:
				if !slices.Contains(finalityStatuses, TxnFinalityStatusWithoutL1(TxnAcceptedOnL2)) {
					return nil
				}

				blockFinalityStatus = TxnFinalityStatusWithoutL1(TxnAcceptedOnL2)

			case core.PreConfirmedBlockVariant:
				if !slices.Contains(finalityStatuses, TxnFinalityStatusWithoutL1(TxnPreConfirmed)) {
					return nil
				}

				blockFinalityStatus = TxnFinalityStatusWithoutL1(TxnPreConfirmed)

			default:
				return fmt.Errorf("unknown pending variant %v", v)
			}

			return processBlockReceipts(
				id,
				w,
				senderAddress,
				block,
				sentCache,
				blockFinalityStatus,
				false,
			)
		},
		onReorg: func(ctx context.Context, id string, _ *subscription, reorg *sync.ReorgBlockRange) error {
			sentCache.Clear()
			return sendReorg(w, reorg, id)
		},
	}
	return h.subscribe(ctx, w, subscriber)
}

// processBlockReceipts streams block events to subscriber
func processBlockReceipts(
	id string,
	w jsonrpc.Conn,
	senderAddress []felt.Felt,
	block *core.Block,
	sentCache *rpccore.SubscriptionCache[SentReceipt, TxnFinalityStatusWithoutL1],
	finalityStatus TxnFinalityStatusWithoutL1,
	isPreLatest bool,
) error {
	for i, txn := range block.Transactions {
		if !filterTxBySender(txn, senderAddress) {
			continue
		}

		adaptedReceipt := AdaptReceipt(
			block.Receipts[i],
			txn,
			TxnFinalityStatus(finalityStatus),
			block.Hash,
			block.Number,
			isPreLatest,
		)

		sentReceipt := SentReceipt{
			TransactionHash:  *adaptedReceipt.Hash,
			TransactionIndex: i,
		}

		if !sentCache.ShouldSend(block.Number, &sentReceipt, &finalityStatus) {
			continue
		}

		sentCache.Put(block.Number, &sentReceipt, &finalityStatus)

		if err := sendTransactionReceipt(w, adaptedReceipt, id); err != nil {
			return err
		}
	}
	return nil
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

type SubscriptionNewTransaction struct {
	Transaction
	FinalityStatus TxnStatusWithoutL1 `json:"finality_status"`
}

// SubscribeNewTransactions Creates a WebSocket stream which will fire events when new transaction are created.
//
// The endpoint receives a vector of finality statuses. An event is fired for each finality status update.
// It is possible for events for pre-confirmed and candidate transactions to be received multiple times, or not at all.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/4e98e3684b50ee9e63b7eeea9412b6a2ed7494ec/api/starknet_ws_api.json#L257
func (h *Handler) SubscribeNewTransactions(
	ctx context.Context,
	finalityStatus []TxnStatusWithoutL1,
	senderAddr []felt.Felt,
) (SubscriptionID, *jsonrpc.Error) {
	w, ok := jsonrpc.ConnFromContext(ctx)
	if !ok {
		return "", jsonrpc.Err(jsonrpc.MethodNotFound, nil)
	}

	if len(senderAddr) > rpccore.MaxEventFilterKeys {
		return "", rpccore.ErrTooManyAddressesInFilter
	}

	if len(finalityStatus) == 0 {
		finalityStatus = []TxnStatusWithoutL1{
			TxnStatusWithoutL1(TxnStatusAcceptedOnL2),
		}
	} else {
		finalityStatus = utils.Set(finalityStatus)
	}

	sentCache := rpccore.NewSubscriptionCache[felt.TransactionHash, TxnStatusWithoutL1]()
	subscriber := subscriber{
		onReorg: func(ctx context.Context, id string, _ *subscription, reorg *sync.ReorgBlockRange) error {
			sentCache.Clear()
			return sendReorg(w, reorg, id)
		},
		onNewHead: func(ctx context.Context, id string, _ *subscription, head *core.Block) error {
			if !slices.Contains(finalityStatus, TxnStatusWithoutL1(TxnStatusAcceptedOnL2)) {
				return nil
			}
			return processBlockTransactions(
				id,
				w,
				senderAddr,
				head,
				sentCache,
				TxnStatusWithoutL1(TxnStatusAcceptedOnL2),
			)
		},
		onPreLatest: func(
			ctx context.Context,
			id string,
			_ *subscription,
			preLatest *core.PreLatest,
		) error {
			if !slices.Contains(finalityStatus, TxnStatusWithoutL1(TxnStatusAcceptedOnL2)) {
				return nil
			}
			return processBlockTransactions(
				id,
				w,
				senderAddr,
				preLatest.Block,
				sentCache,
				TxnStatusWithoutL1(TxnStatusAcceptedOnL2),
			)
		},
		onPendingData: func(ctx context.Context, id string, _ *subscription, pending core.PendingData) error {
			if pending == nil {
				return nil
			}

			switch pending.Variant() {
			case core.PendingBlockVariant:
				if slices.Contains(finalityStatus, TxnStatusWithoutL1(TxnStatusAcceptedOnL2)) {
					return processBlockTransactions(
						id,
						w,
						senderAddr,
						pending.GetBlock(),
						sentCache,
						TxnStatusWithoutL1(TxnStatusAcceptedOnL2),
					)
				}

			case core.PreConfirmedBlockVariant:
				if slices.Contains(finalityStatus, TxnStatusWithoutL1(TxnStatusPreConfirmed)) {
					err := processBlockTransactions(
						id,
						w,
						senderAddr,
						pending.GetBlock(),
						sentCache,
						TxnStatusWithoutL1(TxnStatusPreConfirmed),
					)
					if err != nil {
						return err
					}
				}

				if slices.Contains(finalityStatus, TxnStatusWithoutL1(TxnStatusCandidate)) {
					return processCandidateTransactions(id, w, senderAddr, pending, sentCache)
				}
			}
			return nil
		},
	}
	return h.subscribe(ctx, w, subscriber)
}

// processBlockTransactions streams given block transactions without duplicates
func processBlockTransactions(
	id string,
	w jsonrpc.Conn,
	senderAddr []felt.Felt,
	b *core.Block,
	sentCache *rpccore.SubscriptionCache[felt.TransactionHash, TxnStatusWithoutL1],
	status TxnStatusWithoutL1,
) error {
	for _, txn := range b.Transactions {
		if !filterTxBySender(txn, senderAddr) {
			continue
		}

		if err := sendTransactionWithoutDuplicate(
			w,
			sentCache,
			b.Number,
			txn,
			status,
			id,
		); err != nil {
			return err
		}
	}
	return nil
}

// processCandidateTransactions streams 'CANDIDATE' transactions
func processCandidateTransactions(
	id string,
	w jsonrpc.Conn,
	senderAddr []felt.Felt,
	preConfirmed core.PendingData,
	sentCache *rpccore.SubscriptionCache[felt.TransactionHash, TxnStatusWithoutL1],
) error {
	for _, txn := range preConfirmed.GetCandidateTransaction() {
		if !filterTxBySender(txn, senderAddr) {
			continue
		}

		if err := sendTransactionWithoutDuplicate(
			w,
			sentCache,
			preConfirmed.GetBlock().Number,
			txn,
			TxnStatusWithoutL1(TxnStatusCandidate),
			id,
		); err != nil {
			return err
		}
	}
	return nil
}

// sendTransactionWithoutDuplicate is a helper function that handles transaction deduplication
// and sends the transaction if it hasn't been sent before with the same finality status
func sendTransactionWithoutDuplicate(
	w jsonrpc.Conn,
	sentCache *rpccore.SubscriptionCache[felt.TransactionHash, TxnStatusWithoutL1],
	blockNumber uint64,
	txn core.Transaction,
	finalityStatus TxnStatusWithoutL1,
	id string,
) error {
	txHash := felt.TransactionHash(*txn.Hash())
	if !sentCache.ShouldSend(
		blockNumber,
		&txHash,
		&finalityStatus,
	) {
		return nil
	}

	// Add to cache
	sentCache.Put(blockNumber, &txHash, &finalityStatus)

	response := SubscriptionNewTransaction{
		Transaction:    *AdaptTransaction(txn),
		FinalityStatus: finalityStatus,
	}

	return sendTransaction(w, &response, id)
}

// sendTransaction creates a response and sends it to the client
func sendTransaction(w jsonrpc.Conn, result *SubscriptionNewTransaction, id string) error {
	return sendResponse("starknet_subscriptionNewTransaction", w, id, result)
}

// sendTxnStatus creates a response and sends it to the client
func sendTxnStatus(w jsonrpc.Conn, status SubscriptionTransactionStatus, id string) error {
	return sendResponse("starknet_subscriptionTransactionStatus", w, id, status)
}

// sendTransactionReceipt creates a response and sends it to the client
func sendTransactionReceipt(w jsonrpc.Conn, receipt *TransactionReceipt, id string) error {
	return sendResponse("starknet_subscriptionNewTransactionReceipts", w, id, receipt)
}

// sendEvent creates a response and sends it to the client
func sendEvent(w jsonrpc.Conn, event *SubscriptionEmittedEvent, id string) error {
	return sendResponse("starknet_subscriptionEvents", w, id, event)
}

// sendHeader creates a request and sends it to the client
func sendHeader(w jsonrpc.Conn, header *core.Header, id string) error {
	return sendResponse("starknet_subscriptionNewHeads", w, id, AdaptBlockHeader(header))
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
