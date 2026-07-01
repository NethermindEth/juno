package rpcv9

import (
	"context"
	"iter"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/pending"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/sync"
)

const subscribeEventsChunkSize = 1024

// SubscribeEvents creates a WebSocket stream which will fire events for
// new Starknet events with applied filters
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/c2e93098b9c2ca0423b7f4d15b201f52f22d8c36/api/starknet_ws_api.json#L59
//
//nolint:lll,nolintlint // url exceeds line limit, nolintlint because conflicting line limit with other lint rules
func (h *Handler) SubscribeEvents(
	ctx context.Context,
	fromAddr *felt.Address,
	keys [][]felt.Felt,
	blockID *SubscriptionBlockID,
	finalityStatus *TxnFinalityStatusWithoutL1,
) (SubscriptionID, *jsonrpc.Error) {
	w, ok := jsonrpc.ConnFromContext(ctx)
	if !ok {
		return "", jsonrpc.Err(jsonrpc.MethodNotFound, nil)
	}

	sub, rpcErr := newEventSubscriber(h, w, fromAddr, keys, blockID, finalityStatus)
	if rpcErr != nil {
		return "", rpcErr
	}

	return h.subscribe(ctx, w, sub)
}

type SubscriptionEmittedEvent struct {
	EmittedEvent
	FinalityStatus TxnFinalityStatus `json:"finality_status"`
}

// SentEvent is the dedup key for a pre_confirmed event notification within a single
// round; the deduper scopes it to a (block number, round identifier). Canonical and
// historical events bypass the deduper entirely.
type SentEvent struct {
	TransactionHash  felt.Felt
	TransactionIndex uint
	EventIndex       uint
}

// eventSubscriberState is touched only by its single subscription dispatch
// goroutine, so its deduper is intentionally lock-free. Don't share it across goroutines.
type eventSubscriberState struct {
	handler      *Handler
	conn         jsonrpc.Conn
	l1HeadNumber uint64
	deduper      *rpccore.PreConfirmedDeduper[SentEvent]
	eventMatcher blockchain.EventMatcher
}

func newEventSubscriber(
	handler *Handler,
	conn jsonrpc.Conn,
	fromAddr *felt.Address,
	keys [][]felt.Felt,
	blockID *SubscriptionBlockID,
	finalityStatus *TxnFinalityStatusWithoutL1,
) (subscriber, *jsonrpc.Error) {
	lenKeys := len(keys)
	for _, k := range keys {
		if lenKeys += len(k); lenKeys > rpccore.MaxEventFilterKeys {
			return subscriber{}, rpccore.ErrTooManyKeysInFilter
		}
	}

	requestedHeader, headHeader, rpcErr := handler.resolveBlockRange(blockID)
	if rpcErr != nil {
		return subscriber{}, rpcErr
	}

	l1Head, err := handler.bcReader.L1Head()
	if err != nil {
		return subscriber{}, rpccore.ErrInternal.CloneWithData(err.Error())
	}

	var addresses []felt.Address
	if fromAddr != nil {
		addresses = []felt.Address{*fromAddr}
	}

	state := &eventSubscriberState{
		handler:      handler,
		conn:         conn,
		l1HeadNumber: l1Head.BlockNumber,
		deduper:      rpccore.NewPreConfirmedDeduper[SentEvent](),
		eventMatcher: blockchain.NewEventMatcher(addresses, keys),
	}

	// Historical replay is bounded to the canonical tip even for pre_confirmed
	// subscribers: the pre_confirmed window is owned exclusively by the realtime
	// onPreConfirmed handler, which avoids duplicating the tip during handoff.
	fromBlock := BlockIDFromNumber(requestedHeader.Number)
	toBlock := BlockIDFromNumber(headHeader.Number)

	s := subscriber{
		onStart: func(ctx context.Context, id string, _ *subscription, _ any) error {
			return state.processHistoricalEvents(
				ctx,
				id,
				&fromBlock,
				&toBlock,
				fromAddr,
				keys,
				headHeader.Number,
			)
		},
		onReorg:   state.onReorg,
		onNewHead: state.onNewHead,
	}

	if finalityStatus != nil && *finalityStatus == TxnFinalityStatusWithoutL1(TxnPreConfirmed) {
		s.onPreConfirmed = state.onPreConfirmed
	}

	return s, nil
}

func (s *eventSubscriberState) onReorg(
	_ context.Context,
	id string,
	_ *subscription,
	reorg *sync.ReorgBlockRange,
) error {
	s.deduper.Clear()
	return sendReorg(s.conn, reorg, id)
}

func (s *eventSubscriberState) onNewHead(
	ctx context.Context,
	id string,
	_ *subscription,
	head *core.Block,
) error {
	// Canonical blocks bypass the deduper: they are published once.
	for event := range matchingEvents(&s.eventMatcher, head) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := s.sendFilteredEvent(id, event, TxnAcceptedOnL2); err != nil {
			return err
		}
	}
	return nil
}

func (s *eventSubscriberState) onPreConfirmed(
	ctx context.Context,
	id string,
	_ *subscription,
	preConfirmed *pending.PreConfirmed,
) error {
	block := preConfirmed.GetBlock()
	// The pre_confirmed tip is re-published in full on every delta; skip already-sent
	// events. A same-height round replacement changes BlockIdentifier and re-emits.
	for event := range matchingEvents(&s.eventMatcher, block) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		key := SentEvent{
			TransactionHash:  *event.TransactionHash,
			TransactionIndex: event.TransactionIndex,
			EventIndex:       event.EventIndex,
		}
		if !s.deduper.MarkSent(block.Number, preConfirmed.BlockIdentifier, &key) {
			continue
		}

		if err := s.sendFilteredEvent(id, event, TxnPreConfirmed); err != nil {
			return err
		}
	}
	return nil
}

// matchingEvents yields each event in the block that passes the matcher's
// address/key filter, as a FilteredEvent ready to emit.
func matchingEvents(
	matcher *blockchain.EventMatcher,
	block *core.Block,
) iter.Seq[*blockchain.FilteredEvent] {
	return func(yield func(*blockchain.FilteredEvent) bool) {
		if isMatch := matcher.TestBloom(block.EventsBloom); !isMatch {
			return
		}

		for txIndex, receipt := range block.Receipts {
			for i, event := range receipt.Events {
				if !matcher.MatchesAddress(event.From) {
					continue
				}

				if !matcher.MatchesEventKeys(event.Keys) {
					continue
				}

				filtered := &blockchain.FilteredEvent{
					BlockNumber:      &block.Number,
					BlockHash:        block.Hash,
					TransactionHash:  receipt.TransactionHash,
					TransactionIndex: uint(txIndex),
					EventIndex:       uint(i),
					Event:            event,
				}

				if !yield(filtered) {
					return
				}
			}
		}
	}
}

// processHistoricalEvents queries database for events and stream filtered events.
func (s *eventSubscriberState) processHistoricalEvents(
	ctx context.Context,
	id string,
	from,
	to *BlockID,
	fromAddr *felt.Address,
	keys [][]felt.Felt,
	height uint64,
) error {
	var addresses []felt.Address
	if fromAddr != nil {
		addresses = []felt.Address{*fromAddr}
	}
	filter, err := s.handler.bcReader.EventFilter(
		addresses,
		keys,
		func() (blockchain.PreConfirmedReader, error) {
			chain, err := s.handler.syncReader.PreConfirmedChain()
			if err != nil {
				return nil, err
			}
			return &chain, nil
		},
	)
	if err != nil {
		return err
	}

	defer s.handler.callAndLogErr(filter.Close, "error closing event filter in events subscription")

	err = setEventFilterRange(filter, from, to, height)
	if err != nil {
		return err
	}

	filteredEvents, cToken, err := filter.Events(nil, subscribeEventsChunkSize)
	if err != nil {
		return err
	}

	err = s.sendHistoricalEvents(ctx, id, filteredEvents)
	if err != nil {
		return err
	}

	for !cToken.IsEmpty() {
		filteredEvents, cToken, err = filter.Events(&cToken, subscribeEventsChunkSize)
		if err != nil {
			return err
		}

		err = s.sendHistoricalEvents(ctx, id, filteredEvents)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *eventSubscriberState) sendHistoricalEvents(
	ctx context.Context,
	id string,
	events []blockchain.FilteredEvent,
) error {
	for _, event := range events {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Historical replay is bounded to the canonical tip, so every event
			// here is canonical: L1-finalised at or below the L1 head, else L2.
			finalityStatus := TxnAcceptedOnL2
			if *event.BlockNumber <= s.l1HeadNumber {
				finalityStatus = TxnAcceptedOnL1
			}

			// Historical replay is a one-shot bootstrap with no internal
			// duplicates, so it sends directly without the deduper.
			if err := s.sendFilteredEvent(id, &event, finalityStatus); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *eventSubscriberState) sendFilteredEvent(
	id string,
	event *blockchain.FilteredEvent,
	finalityStatus TxnFinalityStatus,
) error {
	emittedEvent := EmittedEvent{
		BlockNumber:     event.BlockNumber,
		BlockHash:       event.BlockHash,
		TransactionHash: event.TransactionHash,
		Event: &Event{
			From: event.From,
			Keys: event.Keys,
			Data: event.Data,
		},
	}

	response := &SubscriptionEmittedEvent{
		EmittedEvent:   emittedEvent,
		FinalityStatus: finalityStatus,
	}

	return sendEvent(s.conn, response, id)
}

func sendEvent(w jsonrpc.Conn, event *SubscriptionEmittedEvent, id string) error {
	return sendResponse("starknet_subscriptionEvents", w, id, event)
}
