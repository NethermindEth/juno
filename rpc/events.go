package rpc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/feed"
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

/****************************************************
		Events Handlers
*****************************************************/

type SubscriptionEvent byte

const (
	EventNewBlocks SubscriptionEvent = iota + 1
	EventPendingBlocks
	EventL1Blocks
)

func (s *SubscriptionEvent) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case `"newBlocks"`:
		*s = EventNewBlocks
	case `"pendingBlocks"`:
		*s = EventPendingBlocks
	case `"l1Blocks"`:
		*s = EventL1Blocks
	default:
		return fmt.Errorf("unknown subscription event type: %s", string(data))
	}
	return nil
}

func (h *Handler) Subscribe(ctx context.Context, event SubscriptionEvent, withTxs bool) (uint64, *jsonrpc.Error) {
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

	adaptBlock := func(block *core.Block, status BlockStatus) any {
		return adaptBlockWithTxHashes(block, status)
	}
	if withTxs {
		adaptBlock = func(block *core.Block, status BlockStatus) any {
			return adaptBlockWithTxs(block, status)
		}
	}
	switch event {
	case EventNewBlocks:
		subscribe[*core.Block](subscriptionCtx, h.newBlocks.Subscribe(), h, id, sub, func(b *core.Block) (any, error) {
			return adaptBlock(b, BlockAcceptedL2), nil
		})
	case EventPendingBlocks:
		subscribe[*core.Block](subscriptionCtx, h.pendingBlock.Subscribe(), h, id, sub, func(b *core.Block) (any, error) {
			return adaptBlock(b, BlockPending), nil
		})
	case EventL1Blocks:
		if h.l1Reader == nil {
			h.Unsubscribe(ctx, id)
			return 0, jsonrpc.Err(jsonrpc.InternalError, "subscription event not supported")
		}
		subscribe[*core.L1Head](subscriptionCtx, h.l1Heads.Subscribe(), h, id, sub, func(l1Head *core.L1Head) (any, error) {
			block, err := h.bcReader.BlockByNumber(l1Head.BlockNumber)
			if err != nil {
				return nil, fmt.Errorf("get block %d: %v", l1Head.BlockNumber, err)
			}
			return adaptBlock(block, BlockAcceptedL1), nil
		})
	default:
		return 0, jsonrpc.Err(jsonrpc.InternalError, fmt.Sprintf("unknown event type: %d", event))
	}
	return id, nil
}

func adaptBlockWithTxs(block *core.Block, status BlockStatus) *BlockWithTxs {
	txs := make([]*Transaction, len(block.Transactions))
	for index, txn := range block.Transactions {
		txs[index] = AdaptTransaction(txn)
	}

	return &BlockWithTxs{
		Status:       status,
		BlockHeader:  adaptBlockHeader(block.Header),
		Transactions: txs,
	}
}

func adaptBlockWithTxHashes(block *core.Block, status BlockStatus) *BlockWithTxHashes {
	txnHashes := make([]*felt.Felt, len(block.Transactions))
	for index, txn := range block.Transactions {
		txnHashes[index] = txn.Hash()
	}

	return &BlockWithTxHashes{
		Status:      status,
		BlockHeader: adaptBlockHeader(block.Header),
		TxnHashes:   txnHashes,
	}
}

func subscribe[T any](ctx context.Context, sub *feed.Subscription[T], h *Handler,
	id uint64, vsub *subscription, adapt func(T) (any, error),
) {
	vsub.wg.Go(func() {
		defer func() {
			sub.Unsubscribe()
			h.Unsubscribe(ctx, id)
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case v := <-sub.Recv():
				result, err := adapt(v)
				if err != nil {
					h.log.Warnw("Failed to adapt subscription result, closing", "err", err)
					return
				}
				resp, err := json.Marshal(jsonrpc.Request{
					Version: "2.0",
					Method:  "juno_subscription",
					Params: map[string]any{
						"result":       result,
						"subscription": id,
					},
				})
				if err != nil {
					h.log.Warnw("Marshalling subscription reply failed, closing", "err", err)
					return
				}
				if _, err = vsub.conn.Write(resp); err != nil {
					h.log.Warnw("Writing subscription reply failed, closing", "err", err)
					return
				}
			}
		}
	})
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
