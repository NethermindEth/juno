package rpc

import (
	"context"
	"encoding/json"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/jsonrpc"
)

const (
	MaxBlocksBack = 1024
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

type SubscriptionID struct {
	ID uint64 `json:"subscription_id"`
}

/****************************************************
		Events Handlers
*****************************************************/

func (h *Handler) SubscribeNewHeads(ctx context.Context, blockID *BlockID) (*SubscriptionID, *jsonrpc.Error) {
	w, ok := jsonrpc.ConnFromContext(ctx)
	if !ok {
		return nil, jsonrpc.Err(jsonrpc.MethodNotFound, nil)
	}

	startHeader, latestHeader, rpcErr := h.getStartAndLatestHeaders(blockID)
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
	sub.wg.Go(func() {
		defer func() {
			h.unsubscribe(sub, id)
			headerSub.Unsubscribe()
		}()

		newHeadersChan := make(chan *core.Header, MaxBlocksBack)

		sub.wg.Go(func() {
			h.bufferNewHeaders(subscriptionCtx, headerSub, newHeadersChan)
		})

		if err := h.sendHistoricalHeaders(subscriptionCtx, startHeader, latestHeader, w, id); err != nil {
			h.log.Errorw("Error sending old headers", "err", err)
			return
		}

		h.processNewHeaders(subscriptionCtx, newHeadersChan, w, id)
	})

	return &SubscriptionID{ID: id}, nil
}

// getStartAndLatestHeaders gets the start and latest header for the subscription
func (h *Handler) getStartAndLatestHeaders(blockID *BlockID) (*core.Header, *core.Header, *jsonrpc.Error) {
	if blockID == nil || blockID.Latest {
		return nil, nil, nil
	}

	latestHeader, err := h.bcReader.HeadsHeader()
	if err != nil {
		return nil, nil, ErrInternal
	}

	startHeader, rpcErr := h.blockHeaderByID(blockID)
	if rpcErr != nil {
		return nil, nil, rpcErr
	}

	if latestHeader.Number > MaxBlocksBack && startHeader.Number < latestHeader.Number-MaxBlocksBack {
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
	if startHeader == nil {
		return nil
	}

	var err error

	lastHeader := startHeader
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := h.sendHeader(w, lastHeader, id); err != nil {
				return err
			}

			if lastHeader.Number == latestHeader.Number {
				return nil
			}

			lastHeader, err = h.bcReader.BlockHeaderByNumber(lastHeader.Number + 1)
			if err != nil {
				return err
			}
		}
	}
}

func (h *Handler) bufferNewHeaders(ctx context.Context, headerSub *feed.Subscription[*core.Header], newHeadersChan chan<- *core.Header) {
	for {
		select {
		case <-ctx.Done():
			return
		case header := <-headerSub.Recv():
			newHeadersChan <- header
		}
	}
}

func (h *Handler) processNewHeaders(ctx context.Context, newHeadersChan <-chan *core.Header, w jsonrpc.Conn, id uint64) {
	for {
		select {
		case <-ctx.Done():
			return
		case header := <-newHeadersChan:
			if err := h.sendHeader(w, header, id); err != nil {
				h.log.Warnw("Error sending header", "err", err)
				return
			}
		}
	}
}

// sendHeader creates a request and sends it to the client
func (h *Handler) sendHeader(w jsonrpc.Conn, header *core.Header, id uint64) error {
	resp, err := json.Marshal(jsonrpc.Request{
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
