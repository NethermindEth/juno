package rpcv6

import (
	"context"
	"encoding/json"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	rpccore "github.com/NethermindEth/juno/rpc/rpccore"
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
	Events            []EmittedEvent `json:"events"`
	ContinuationToken string         `json:"continuation_token,omitempty"`
}

/****************************************************
		Events Handlers
*****************************************************/

// Events gets the events matching a filter
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/94a969751b31f5d3e25a0c6850c723ddadeeb679/api/starknet_api_openrpc.json#L642
func (h *Handler) Events(args EventsArg) (*EventsChunk, *jsonrpc.Error) {
	if args.ChunkSize > rpccore.MaxEventChunkSize {
		return nil, rpccore.ErrPageSizeTooBig
	} else {
		lenKeys := len(args.Keys)
		for _, keys := range args.Keys {
			lenKeys += len(keys)
		}
		if lenKeys > rpccore.MaxEventFilterKeys {
			return nil, rpccore.ErrTooManyKeysInFilter
		}
	}

	height, err := h.bcReader.Height()
	if err != nil {
		return nil, rpccore.ErrInternal
	}

	var addresses []felt.Address
	if args.EventFilter.Address != nil {
		addresses = []felt.Address{felt.Address(*args.EventFilter.Address)}
	}
	filter, err := h.bcReader.EventFilter(
		addresses,
		args.EventFilter.Keys,
		h.PendingData,
	)
	if err != nil {
		return nil, rpccore.ErrInternal
	}
	filter = filter.WithLimit(h.filterLimit)
	defer h.callAndLogErr(filter.Close, "Error closing event filter in events")

	var cToken *blockchain.ContinuationToken
	if args.ContinuationToken != "" {
		cToken = new(blockchain.ContinuationToken)
		if err = cToken.FromString(args.ContinuationToken); err != nil {
			return nil, rpccore.ErrInvalidContinuationToken
		}
	}

	if err = setEventFilterRange(
		filter,
		args.EventFilter.FromBlock,
		args.EventFilter.ToBlock,
		height,
	); err != nil {
		return nil, rpccore.ErrBlockNotFound
	}

	filteredEvents, cTokenValue, err := filter.Events(cToken, args.ChunkSize)
	if err != nil {
		return nil, rpccore.ErrInternal
	}

	emittedEvents := make([]EmittedEvent, len(filteredEvents))
	for i, fEvent := range filteredEvents {
		var blockNumber *uint64
		if fEvent.BlockHash != nil {
			blockNumber = fEvent.BlockNumber
		}
		emittedEvents[i] = EmittedEvent{
			BlockNumber:     blockNumber,
			BlockHash:       fEvent.BlockHash,
			TransactionHash: fEvent.TransactionHash,
			Event: &Event{
				From: fEvent.From,
				Keys: fEvent.Keys,
				Data: fEvent.Data,
			},
		}
	}

	cTokenStr := ""
	if !cTokenValue.IsEmpty() {
		cTokenStr = cTokenValue.String()
	}
	return &EventsChunk{Events: emittedEvents, ContinuationToken: cTokenStr}, nil
}

func setEventFilterRange(
	filter blockchain.EventFilterer,
	fromID,
	toID *BlockID,
	latestHeight uint64,
) error {
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
			if filterRange == blockchain.EventFilterTo {
				return filter.SetRangeEndBlockByNumber(
					filterRange,
					min(id.Number, latestHeight),
				)
			}
			return filter.SetRangeEndBlockByNumber(filterRange, id.Number)
		}
	}
	if err := set(blockchain.EventFilterFrom, fromID); err != nil {
		return err
	}
	return set(blockchain.EventFilterTo, toID)
}

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
						"result":       adaptBlockHeader(header.Header),
						"subscription": id,
					},
				})
				if err != nil {
					h.log.Warn("Error marshalling a subscription reply", utils.SugaredFields("err", err)...)
					return
				}
				if _, err = w.Write(resp); err != nil {
					h.log.Warn("Error writing a subscription reply", utils.SugaredFields("err", err)...)
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

// unsubscribe assumes h.mu is unlocked. It releases all subscription resources.
func (h *Handler) unsubscribe(sub *subscription, id uint64) {
	sub.cancel()
	h.subscriptions.Delete(id)
}
