package rpc

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
)

const subscribeEventsChunkSize = 1024

type SubscriptionResponse struct {
	Version string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

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

	var requestedHeader *core.Header
	headHeader, err := h.bcReader.HeadsHeader()
	if err != nil {
		return nil, ErrInternal.CloneWithData(err.Error())
	}

	if blockID == nil {
		requestedHeader = headHeader
	} else {
		if blockID.Pending {
			return nil, ErrCallOnPending
		}

		var rpcErr *jsonrpc.Error
		requestedHeader, rpcErr = h.blockHeaderByID(blockID)
		if rpcErr != nil {
			return nil, rpcErr
		}

		if headHeader.Number >= maxBlocksBack && requestedHeader.Number <= headHeader.Number-maxBlocksBack {
			return nil, ErrTooManyBlocksBack
		}
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
	sub.wg.Go(func() {
		defer func() {
			h.unsubscribe(sub, id)
			headerSub.Unsubscribe()
		}()

		// The specification doesn't enforce ordering of events therefore events from new blocks can be sent before
		// old blocks.
		// Todo: see if sub's wg can be used?
		wg := sync.WaitGroup{}
		wg.Add(1)

		go func() {
			defer wg.Done()

			for {
				select {
				case <-subscriptionCtx.Done():
					return
				case header := <-headerSub.Recv():

					h.processEvents(subscriptionCtx, w, id, header.Number, header.Number, fromAddr, keys)
				}
			}
		}()

		h.processEvents(subscriptionCtx, w, id, requestedHeader.Number, headHeader.Number, fromAddr, keys)

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
