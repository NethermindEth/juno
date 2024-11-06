package rpc

import (
	"context"
	"encoding/json"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
)

type EventsArg struct {
	EventFilter
	ResultPageRequest
}

type EventFilter struct {
	FromBlock *BlockID      `json:"from_block"`
	ToBlock   *BlockID      `json:"to_block"`
	Address   *felt.Felt    `json:"address"`
	Keys      [][]felt.Felt `json:"keys"`
}

type ResultPageRequest struct {
	ContinuationToken string `json:"continuation_token"`
	ChunkSize         uint64 `json:"chunk_size" validate:"min=1"`
}

type Event struct {
	From *felt.Felt   `json:"from_address,omitempty"`
	Keys []*felt.Felt `json:"keys"`
	Data []*felt.Felt `json:"data"`
}

type EmittedEvent struct {
	*Event
	BlockNumber     *uint64    `json:"block_number,omitempty"`
	BlockHash       *felt.Felt `json:"block_hash,omitempty"`
	TransactionHash *felt.Felt `json:"transaction_hash"`
}

type EventsChunk struct {
	Events            []*EmittedEvent `json:"events"`
	ContinuationToken string          `json:"continuation_token,omitempty"`
}

type EventSubscription struct {
	From      *felt.Felt    `json:"from_address"`
	Keys      [][]felt.Felt `json:"keys"`
	FromBlock *BlockID      `json:"block"`
}

type SubscriptionID struct {
	ID uint64 `json:"subscription_id"`
}

/****************************************************
		Events Handlers
*****************************************************/

func (h *Handler) SubscribeNewHeads(ctx context.Context) (uint64, *jsonrpc.Error) {
	w, ok := jsonrpc.ConnFromContext(ctx)
	if !ok {
		return 0, jsonrpc.Err(jsonrpc.MethodNotFound, nil)
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
			headerSub.Unsubscribe()
			h.unsubscribe(sub, id)
		}()
		for {
			select {
			case <-subscriptionCtx.Done():
				return
			case header := <-headerSub.Recv():
				resp, err := json.Marshal(jsonrpc.Request{
					Version: "2.0",
					Method:  "juno_subscribeNewHeads",
					Params: map[string]any{
						"result":       adaptBlockHeader(header),
						"subscription": id,
					},
				})
				if err != nil {
					h.log.Warnw("Error marshalling a subscription reply", "err", err)
					return
				}
				if _, err = w.Write(resp); err != nil {
					h.log.Warnw("Error writing a subscription reply", "err", err)
					return
				}
			}
		}
	})
	return id, nil
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

const subscribeEventsChunkSize = 1024

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
		requestedHeader, rpcErr := h.blockHeaderByID(blockID)
		if rpcErr != nil {
			return nil, rpcErr
		}

		// Todo: should the pending block be included in the head count?
		if requestedHeader.Number >= maxBlocksBack && requestedHeader.Number <= headHeader.Number-maxBlocksBack {
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
		// Todo: DRY
		sub.wg.Go(func() {
			for {
				select {
				case <-subscriptionCtx.Done():
					return
				case header := <-headerSub.Recv():
					filter, err := h.bcReader.EventFilter(fromAddr, keys)
					if err != nil {
						h.log.Warnw("Error creating event filter", "err", err)
						return
					}
					defer h.callAndLogErr(filter.Close, "Error closing event filter in events subscription")

					if err = setEventFilterRange(filter, &BlockID{Number: header.Number},
						&BlockID{Number: header.Number}, header.Number); err != nil {
						h.log.Warnw("Error setting event filter range", "err", err)
						return
					}

					var cToken *blockchain.ContinuationToken
					filteredEvents, cToken, err := filter.Events(cToken, subscribeEventsChunkSize)
					if err != nil {
						h.log.Warnw("Error filtering events", "err", err)
						return
					}

					err = sendEvents(subscriptionCtx, w, filteredEvents, id)
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

						err = sendEvents(subscriptionCtx, w, filteredEvents, id)
						if err != nil {
							h.log.Warnw("Error sending events", "err", err)
							return
						}
					}
				}
			}
		})

		filter, err := h.bcReader.EventFilter(fromAddr, keys)
		if err != nil {
			h.log.Warnw("Error creating event filter", "err", err)
			return
		}
		defer h.callAndLogErr(filter.Close, "Error closing event filter in events subscription")

		if err = setEventFilterRange(filter, &BlockID{Number: requestedHeader.Number},
			&BlockID{Number: headHeader.Number}, headHeader.Number); err != nil {
			h.log.Warnw("Error setting event filter range", "err", err)
			return
		}

		var cToken *blockchain.ContinuationToken
		filteredEvents, cToken, err := filter.Events(cToken, subscribeEventsChunkSize)
		if err != nil {
			h.log.Warnw("Error filtering events", "err", err)
			return
		}

		err = sendEvents(subscriptionCtx, w, filteredEvents, id)
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

			err = sendEvents(subscriptionCtx, w, filteredEvents, id)
			if err != nil {
				h.log.Warnw("Error sending events", "err", err)
				return
			}
		}
	})

	return &SubscriptionID{ID: id}, nil
}

func sendEvents(ctx context.Context, w jsonrpc.Conn, events []*blockchain.FilteredEvent, id uint64) error {
	for _, event := range events {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Pending block doesn't have a number
			var blockNumber *uint64
			if event.BlockHash != nil {
				blockNumber = &(event.BlockNumber)
			}
			emittedEvent := &EmittedEvent{
				BlockNumber:     blockNumber,
				BlockHash:       event.BlockHash,
				TransactionHash: event.TransactionHash,
				Event: &Event{
					From: event.From,
					Keys: event.Keys,
					Data: event.Data,
				},
			}

			resp, err := json.Marshal(jsonrpc.Request{
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
			return err
		}
	}
	return nil
}

// Events gets the events matching a filter
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/94a969751b31f5d3e25a0c6850c723ddadeeb679/api/starknet_api_openrpc.json#L642
func (h *Handler) Events(args EventsArg) (*EventsChunk, *jsonrpc.Error) {
	if args.ChunkSize > maxEventChunkSize {
		return nil, ErrPageSizeTooBig
	} else {
		lenKeys := len(args.Keys)
		for _, keys := range args.Keys {
			lenKeys += len(keys)
		}
		if lenKeys > maxEventFilterKeys {
			return nil, ErrTooManyKeysInFilter
		}
	}

	height, err := h.bcReader.Height()
	if err != nil {
		return nil, ErrInternal
	}

	filter, err := h.bcReader.EventFilter(args.EventFilter.Address, args.EventFilter.Keys)
	if err != nil {
		return nil, ErrInternal
	}
	filter = filter.WithLimit(h.filterLimit)
	defer h.callAndLogErr(filter.Close, "Error closing event filter in events")

	var cToken *blockchain.ContinuationToken
	if args.ContinuationToken != "" {
		cToken = new(blockchain.ContinuationToken)
		if err = cToken.FromString(args.ContinuationToken); err != nil {
			return nil, ErrInvalidContinuationToken
		}
	}

	if err = setEventFilterRange(filter, args.EventFilter.FromBlock, args.EventFilter.ToBlock, height); err != nil {
		return nil, ErrBlockNotFound
	}

	filteredEvents, cToken, err := filter.Events(cToken, args.ChunkSize)
	if err != nil {
		return nil, ErrInternal
	}

	emittedEvents := make([]*EmittedEvent, 0, len(filteredEvents))
	for _, fEvent := range filteredEvents {
		var blockNumber *uint64
		if fEvent.BlockHash != nil {
			blockNumber = &(fEvent.BlockNumber)
		}
		emittedEvents = append(emittedEvents, &EmittedEvent{
			BlockNumber:     blockNumber,
			BlockHash:       fEvent.BlockHash,
			TransactionHash: fEvent.TransactionHash,
			Event: &Event{
				From: fEvent.From,
				Keys: fEvent.Keys,
				Data: fEvent.Data,
			},
		})
	}

	cTokenStr := ""
	if cToken != nil {
		cTokenStr = cToken.String()
	}
	return &EventsChunk{Events: emittedEvents, ContinuationToken: cTokenStr}, nil
}

// unsubscribe assumes h.mu is unlocked. It releases all subscription resources.
func (h *Handler) unsubscribe(sub *subscription, id uint64) {
	sub.cancel()
	h.mu.Lock()
	delete(h.subscriptions, id)
	h.mu.Unlock()
}

func setEventFilterRange(filter *blockchain.EventFilter, fromID, toID *BlockID, latestHeight uint64) error {
	set := func(filterRange blockchain.EventFilterRange, id *BlockID) error {
		if id == nil {
			return nil
		}

		switch {
		case id.Latest:
			return filter.SetRangeEndBlockByNumber(filterRange, latestHeight)
		case id.Hash != nil:
			return filter.SetRangeEndBlockByHash(filterRange, id.Hash)
		case id.Pending:
			return filter.SetRangeEndBlockByNumber(filterRange, latestHeight+1)
		default:
			return filter.SetRangeEndBlockByNumber(filterRange, id.Number)
		}
	}
	if err := set(blockchain.EventFilterFrom, fromID); err != nil {
		return err
	}
	return set(blockchain.EventFilterTo, toID)
}
