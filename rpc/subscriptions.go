package rpc

import (
	"context"
	"encoding/json"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/sync"
	"github.com/sourcegraph/conc"
)

const subscribeEventsChunkSize = 1024

type SubscriptionResponse struct {
	Version string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

// SubscribeEvents creates a WebSocket stream which will fire events for new Starknet events with applied filters
func (h *Handler) SubscribeEvents(ctx context.Context, fromAddr *felt.Felt, keys [][]felt.Felt,
	blockID *BlockID,
) (*SubscriptionID, *jsonrpc.Error) {
	w, ok := jsonrpc.ConnFromContext(ctx)
	if !ok {
		return nil, jsonrpc.Err(jsonrpc.MethodNotFound, nil)
	}

	lenKeys := len(keys)
	for _, k := range keys {
		lenKeys += len(k)
	}
	if lenKeys > maxEventFilterKeys {
		return nil, ErrTooManyKeysInFilter
	}

	requestedHeader, headHeader, rpcErr := h.resolveBlockRange(blockID)
	if rpcErr != nil {
		return nil, rpcErr
	}

	id := h.idgen()
	subscriptionCtx, subscriptionCtxCancel := context.WithCancel(ctx)
	sub := &subscription{
		cancel: subscriptionCtxCancel,
		conn:   w,
	}
	h.mu.Lock()
	h.subscriptions[id] = sub
	h.mu.Unlock()

	headerSub := h.newHeads.Subscribe()
	reorgSub := h.reorgs.Subscribe() // as per the spec, reorgs are also sent in the events subscription
	sub.wg.Go(func() {
		defer func() {
			h.unsubscribe(sub, id)
			headerSub.Unsubscribe()
			reorgSub.Unsubscribe()
		}()

		// The specification doesn't enforce ordering of events therefore events from new blocks can be sent before
		// old blocks.
		var wg conc.WaitGroup
		wg.Go(func() {
			for {
				select {
				case <-subscriptionCtx.Done():
					return
				case header := <-headerSub.Recv():
					h.processEvents(subscriptionCtx, w, id, header.Number, header.Number, fromAddr, keys)
				}
			}
		})

		wg.Go(func() {
			h.processReorgs(subscriptionCtx, reorgSub, w, id)
		})

		wg.Go(func() {
			h.processEvents(subscriptionCtx, w, id, requestedHeader.Number, headHeader.Number, fromAddr, keys)
		})

		wg.Wait()
	})

	return &SubscriptionID{ID: id}, nil
}

func (h *Handler) processEvents(ctx context.Context, w jsonrpc.Conn, id, from, to uint64, fromAddr *felt.Felt, keys [][]felt.Felt) {
	filter, err := h.bcReader.EventFilter(fromAddr, keys)
	if err != nil {
		h.log.Warnw("Error creating event filter", "err", err)
		return
	}

	defer h.callAndLogErr(filter.Close, "Error closing event filter in events subscription")

	if err = setEventFilterRange(filter, &BlockID{Number: from}, &BlockID{Number: to}, to); err != nil {
		h.log.Warnw("Error setting event filter range", "err", err)
		return
	}

	filteredEvents, cToken, err := filter.Events(nil, subscribeEventsChunkSize)
	if err != nil {
		h.log.Warnw("Error filtering events", "err", err)
		return
	}

	err = sendEvents(ctx, w, filteredEvents, id)
	if err != nil {
		h.log.Warnw("Error sending events", "err", err)
		return
	}

	for cToken != nil {
		filteredEvents, cToken, err = filter.Events(cToken, subscribeEventsChunkSize)
		if err != nil {
			h.log.Warnw("Error filtering events", "err", err)
			return
		}

		err = sendEvents(ctx, w, filteredEvents, id)
		if err != nil {
			h.log.Warnw("Error sending events", "err", err)
			return
		}
	}
}

func sendEvents(ctx context.Context, w jsonrpc.Conn, events []*blockchain.FilteredEvent, id uint64) error {
	for _, event := range events {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			emittedEvent := &EmittedEvent{
				BlockNumber:     &event.BlockNumber, // This always be filled as subscribeEvents cannot be called on pending block
				BlockHash:       event.BlockHash,
				TransactionHash: event.TransactionHash,
				Event: &Event{
					From: event.From,
					Keys: event.Keys,
					Data: event.Data,
				},
			}

			resp, err := json.Marshal(SubscriptionResponse{
				Version: "2.0",
				Method:  "starknet_subscriptionEvents",
				Params: map[string]any{
					"subscription_id": id,
					"result":          emittedEvent,
				},
			})
			if err != nil {
				return err
			}

			_, err = w.Write(resp)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// SubscribeNewHeads creates a WebSocket stream which will fire events when a new block header is added.
func (h *Handler) SubscribeNewHeads(ctx context.Context, blockID *BlockID) (*SubscriptionID, *jsonrpc.Error) {
	w, ok := jsonrpc.ConnFromContext(ctx)
	if !ok {
		return nil, jsonrpc.Err(jsonrpc.MethodNotFound, nil)
	}

	startHeader, latestHeader, rpcErr := h.resolveBlockRange(blockID)
	if rpcErr != nil {
		return nil, rpcErr
	}

	id := h.idgen()
	subscriptionCtx, subscriptionCtxCancel := context.WithCancel(ctx)
	sub := &subscription{
		cancel: subscriptionCtxCancel,
		conn:   w,
	}
	h.mu.Lock()
	h.subscriptions[id] = sub
	h.mu.Unlock()

	headerSub := h.newHeads.Subscribe()
	reorgSub := h.reorgs.Subscribe() // as per the spec, reorgs are also sent in the new heads subscription
	sub.wg.Go(func() {
		defer func() {
			h.unsubscribe(sub, id)
			headerSub.Unsubscribe()
			reorgSub.Unsubscribe()
		}()

		var wg conc.WaitGroup

		wg.Go(func() {
			if err := h.sendHistoricalHeaders(subscriptionCtx, startHeader, latestHeader, w, id); err != nil {
				h.log.Errorw("Error sending old headers", "err", err)
				return
			}
		})

		wg.Go(func() {
			h.processReorgs(subscriptionCtx, reorgSub, w, id)
		})

		wg.Go(func() {
			h.processNewHeaders(subscriptionCtx, headerSub, w, id)
		})

		wg.Wait()
	})

	return &SubscriptionID{ID: id}, nil
}

// SubscribePendingTxs creates a WebSocket stream which will fire events when a new pending transaction is added.
// The getDetails flag controls if the response will contain the transaction details or just the transaction hashes.
// The senderAddr flag is used to filter the transactions by sender address.
func (h *Handler) SubscribePendingTxs(ctx context.Context, getDetails *bool, senderAddr []felt.Felt) (*SubscriptionID, *jsonrpc.Error) {
	w, ok := jsonrpc.ConnFromContext(ctx)
	if !ok {
		return nil, jsonrpc.Err(jsonrpc.MethodNotFound, nil)
	}

	if len(senderAddr) > maxEventFilterKeys {
		return nil, ErrTooManyAddressesInFilter
	}

	id := h.idgen()
	subscriptionCtx, subscriptionCtxCancel := context.WithCancel(ctx)
	sub := &subscription{
		cancel: subscriptionCtxCancel,
		conn:   w,
	}
	h.mu.Lock()
	h.subscriptions[id] = sub
	h.mu.Unlock()

	pendingTxsSub := h.pendingTxs.Subscribe()
	sub.wg.Go(func() {
		defer func() {
			h.unsubscribe(sub, id)
			pendingTxsSub.Unsubscribe()
		}()

		h.processPendingTxs(subscriptionCtx, getDetails != nil && *getDetails, senderAddr, pendingTxsSub, w, id)
	})

	return &SubscriptionID{ID: id}, nil
}

func (h *Handler) processPendingTxs(
	ctx context.Context,
	getDetails bool,
	senderAddr []felt.Felt,
	pendingTxsSub *feed.Subscription[[]core.Transaction],
	w jsonrpc.Conn,
	id uint64,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case pendingTxs := <-pendingTxsSub.Recv():
			filteredTxs := h.filterTxs(pendingTxs, getDetails, senderAddr)
			if err := h.sendPendingTxs(w, filteredTxs, id); err != nil {
				h.log.Warnw("Error sending pending transactions", "err", err)
				return
			}
		}
	}
}

// filterTxs filters the transactions based on the getDetails flag.
// If getDetails is true, response will contain the transaction details.
// If getDetails is false, response will only contain the transaction hashes.
func (h *Handler) filterTxs(pendingTxs []core.Transaction, getDetails bool, senderAddr []felt.Felt) interface{} {
	if getDetails {
		return h.filterTxDetails(pendingTxs, senderAddr)
	}
	return h.filterTxHashes(pendingTxs, senderAddr)
}

func (h *Handler) filterTxDetails(pendingTxs []core.Transaction, senderAddr []felt.Felt) []*Transaction {
	filteredTxs := make([]*Transaction, 0, len(pendingTxs))
	for _, txn := range pendingTxs {
		if h.filterTxBySender(txn, senderAddr) {
			filteredTxs = append(filteredTxs, AdaptTransaction(txn))
		}
	}
	return filteredTxs
}

func (h *Handler) filterTxHashes(pendingTxs []core.Transaction, senderAddr []felt.Felt) []felt.Felt {
	filteredTxHashes := make([]felt.Felt, 0, len(pendingTxs))
	for _, txn := range pendingTxs {
		if h.filterTxBySender(txn, senderAddr) {
			filteredTxHashes = append(filteredTxHashes, *txn.Hash())
		}
	}
	return filteredTxHashes
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

func (h *Handler) sendPendingTxs(w jsonrpc.Conn, result interface{}, id uint64) error {
	resp, err := json.Marshal(SubscriptionResponse{
		Version: "2.0",
		Method:  "starknet_subscriptionPendingTransactions",
		Params: map[string]interface{}{
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

// resolveBlockRange returns the start and latest headers based on the blockID.
// It will also do some sanity checks and return errors if the blockID is invalid.
func (h *Handler) resolveBlockRange(blockID *BlockID) (*core.Header, *core.Header, *jsonrpc.Error) {
	latestHeader, err := h.bcReader.HeadsHeader()
	if err != nil {
		return nil, nil, ErrInternal.CloneWithData(err.Error())
	}

	if blockID == nil || blockID.Latest {
		return latestHeader, latestHeader, nil
	}

	if blockID.Pending {
		return nil, nil, ErrCallOnPending
	}

	startHeader, rpcErr := h.blockHeaderByID(blockID)
	if rpcErr != nil {
		return nil, nil, rpcErr
	}

	if latestHeader.Number >= maxBlocksBack && startHeader.Number <= latestHeader.Number-maxBlocksBack {
		return nil, nil, ErrTooManyBlocksBack
	}

	return startHeader, latestHeader, nil
}

// sendHistoricalHeaders sends a range of headers from the start header until the latest header
func (h *Handler) sendHistoricalHeaders(
	ctx context.Context,
	startHeader *core.Header,
	latestHeader *core.Header,
	w jsonrpc.Conn,
	id uint64,
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
			if err := h.sendHeader(w, curHeader, id); err != nil {
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

func (h *Handler) processNewHeaders(ctx context.Context, headerSub *feed.Subscription[*core.Header], w jsonrpc.Conn, id uint64) {
	for {
		select {
		case <-ctx.Done():
			return
		case header := <-headerSub.Recv():
			if err := h.sendHeader(w, header, id); err != nil {
				h.log.Warnw("Error sending header", "err", err)
				return
			}
		}
	}
}

// sendHeader creates a request and sends it to the client
func (h *Handler) sendHeader(w jsonrpc.Conn, header *core.Header, id uint64) error {
	resp, err := json.Marshal(SubscriptionResponse{
		Version: "2.0",
		Method:  "starknet_subscriptionNewHeads",
		Params: map[string]any{
			"subscription_id": id,
			"result":          adaptBlockHeader(header),
		},
	})
	if err != nil {
		return err
	}
	_, err = w.Write(resp)
	return err
}

func (h *Handler) processReorgs(ctx context.Context, reorgSub *feed.Subscription[*sync.ReorgData], w jsonrpc.Conn, id uint64) {
	for {
		select {
		case <-ctx.Done():
			return
		case reorg := <-reorgSub.Recv():
			if err := h.sendReorg(w, reorg, id); err != nil {
				h.log.Warnw("Error sending reorg", "err", err)
				return
			}
		}
	}
}

func (h *Handler) sendReorg(w jsonrpc.Conn, reorg *sync.ReorgData, id uint64) error {
	resp, err := json.Marshal(jsonrpc.Request{
		Version: "2.0",
		Method:  "starknet_subscriptionReorg",
		Params: map[string]any{
			"subscription_id": id,
			"result":          reorg,
		},
	})
	if err != nil {
		return err
	}
	_, err = w.Write(resp)
	return err
}

func (h *Handler) Unsubscribe(ctx context.Context, id uint64) (bool, *jsonrpc.Error) {
	w, ok := jsonrpc.ConnFromContext(ctx)
	if !ok {
		return false, jsonrpc.Err(jsonrpc.MethodNotFound, nil)
	}
	h.mu.Lock()
	sub, ok := h.subscriptions[id]
	h.mu.Unlock() // Don't defer since h.unsubscribe acquires the lock.
	if !ok || !sub.conn.Equal(w) {
		return false, ErrSubscriptionNotFound
	}
	sub.cancel()
	sub.wg.Wait() // Let the subscription finish before responding.
	return true, nil
}
