package rpcv10

import (
	"context"
	"fmt"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
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
	fromAddrs addressList,
	keys [][]felt.Felt,
	blockID *rpcv9.SubscriptionBlockID,
	finalityStatus *rpcv9.TxnFinalityStatusWithoutL1,
) (SubscriptionID, *jsonrpc.Error) {
	w, ok := jsonrpc.ConnFromContext(ctx)
	if !ok {
		return "", jsonrpc.Err(jsonrpc.MethodNotFound, nil)
	}

	sub, rpcErr := newEventSubscriber(h, w, fromAddrs, keys, blockID, finalityStatus)
	if rpcErr != nil {
		return "", rpcErr
	}

	return h.subscribe(ctx, w, sub)
}

type SubscriptionEmittedEvent struct {
	EmittedEvent
	FinalityStatus rpcv9.TxnFinalityStatus `json:"finality_status"`
}

type SentEvent struct {
	TransactionHash  felt.Felt
	TransactionIndex uint
	EventIndex       uint
}

type eventSubscriberState struct {
	handler *Handler
	conn    jsonrpc.Conn

	// filter parameters
	l1HeadNumber uint64
	sentCache    *rpccore.SubscriptionCache[SentEvent, rpcv9.TxnFinalityStatus]
	eventMatcher blockchain.EventMatcher
}

func newEventSubscriber(
	handler *Handler,
	conn jsonrpc.Conn,
	fromAddrs []felt.Address,
	keys [][]felt.Felt,
	blockID *rpcv9.SubscriptionBlockID,
	finalityStatus *rpcv9.TxnFinalityStatusWithoutL1,
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

	state := &eventSubscriberState{
		handler:      handler,
		conn:         conn,
		l1HeadNumber: l1Head.BlockNumber,
		sentCache:    rpccore.NewSubscriptionCache[SentEvent, rpcv9.TxnFinalityStatus](),
		eventMatcher: blockchain.NewEventMatcher(fromAddrs, keys),
	}

	fromBlock := rpcv9.BlockIDFromNumber(requestedHeader.Number)
	var toBlock rpcv9.BlockID
	if finalityStatus != nil &&
		*finalityStatus == rpcv9.TxnFinalityStatusWithoutL1(rpcv9.TxnPreConfirmed) {
		toBlock = rpcv9.BlockIDPreConfirmed()
	} else {
		toBlock = rpcv9.BlockIDFromNumber(headHeader.Number)
	}

	s := subscriber{
		onStart: func(ctx context.Context, id string, _ *subscription, _ any) error {
			return state.processHistoricalEvents(
				ctx, id,
				&fromBlock,
				&toBlock,
				fromAddrs,
				keys,
				headHeader.Number,
			)
		},
		onReorg:     state.onReorg,
		onNewHead:   state.onNewHead,
		onPreLatest: state.onPreLatest,
	}

	if finalityStatus != nil &&
		*finalityStatus == rpcv9.TxnFinalityStatusWithoutL1(rpcv9.TxnPreConfirmed) {
		s.onPendingData = state.onPendingData
	}

	return s, nil
}

func (s *eventSubscriberState) onReorg(
	_ context.Context,
	id string,
	_ *subscription,
	reorg *sync.ReorgBlockRange,
) error {
	s.sentCache.Clear()
	return sendReorg(s.conn, reorg, id)
}

func (s *eventSubscriberState) onNewHead(
	ctx context.Context,
	id string,
	_ *subscription,
	head *core.Block,
) error {
	return s.processBlock(ctx, id, head, rpcv9.TxnAcceptedOnL2)
}

func (s *eventSubscriberState) onPreLatest(
	ctx context.Context,
	id string,
	_ *subscription,
	preLatest *core.PreLatest,
) error {
	return s.processBlock(ctx, id, preLatest.Block, rpcv9.TxnAcceptedOnL2)
}

func (s *eventSubscriberState) onPendingData(
	ctx context.Context,
	id string,
	_ *subscription,
	pending core.PendingData,
) error {
	if pending.Variant() != core.PreConfirmedBlockVariant {
		return fmt.Errorf("unexpected pending data variant %v", pending.Variant())
	}

	return s.processBlock(ctx, id, pending.GetBlock(), rpcv9.TxnPreConfirmed)
}

func (s *eventSubscriberState) processBlock(
	ctx context.Context,
	id string,
	block *core.Block,
	finalityStatus rpcv9.TxnFinalityStatus,
) error {
	if isMatch := s.eventMatcher.TestBloom(block.EventsBloom); !isMatch {
		return nil
	}

	for txIndex, receipt := range block.Receipts {
		for i, event := range receipt.Events {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if !s.eventMatcher.MatchesAddress(event.From) {
				continue
			}

			if !s.eventMatcher.MatchesEventKeys(event.Keys) {
				continue
			}

			event := blockchain.FilteredEvent{
				BlockNumber:      &block.Number,
				BlockHash:        block.Hash,
				TransactionHash:  receipt.TransactionHash,
				TransactionIndex: uint(txIndex),
				EventIndex:       uint(i),
				Event:            event,
			}

			err := s.sendEventWithoutDuplicate(
				id,
				&event,
				finalityStatus,
				block.Number,
			)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// processHistoricalEvents queries database for events and stream filtered events.
func (s *eventSubscriberState) processHistoricalEvents(
	ctx context.Context,
	id string,
	from, to *rpcv9.BlockID,
	fromAddrs []felt.Address,
	keys [][]felt.Felt,
	height uint64,
) error {
	filter, err := s.handler.bcReader.EventFilter(fromAddrs, keys, s.handler.PendingData)
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

	err = s.sendHistoricalEvents(ctx, id, filteredEvents, height)
	if err != nil {
		return err
	}

	for !cToken.IsEmpty() {
		filteredEvents, cToken, err = filter.Events(&cToken, subscribeEventsChunkSize)
		if err != nil {
			return err
		}

		err = s.sendHistoricalEvents(ctx, id, filteredEvents, height)
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
	height uint64,
) error {
	for _, event := range events {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			var finalityStatus rpcv9.TxnFinalityStatus
			switch {
			case *event.BlockNumber > height: // pre_confirmed or pre_latest block
				if event.BlockParentHash == nil {
					finalityStatus = rpcv9.TxnPreConfirmed
				} else {
					finalityStatus = rpcv9.TxnAcceptedOnL2
				}
			case *event.BlockNumber <= s.l1HeadNumber:
				finalityStatus = rpcv9.TxnAcceptedOnL1
			default: // Canonical block not finalised on L1
				finalityStatus = rpcv9.TxnAcceptedOnL2
			}

			if err := s.sendEventWithoutDuplicate(
				id,
				&event,
				finalityStatus,
				*event.BlockNumber,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *eventSubscriberState) sendEventWithoutDuplicate(
	id string,
	event *blockchain.FilteredEvent,
	finalityStatus rpcv9.TxnFinalityStatus,
	blockNum uint64,
) error {
	sentEvent := SentEvent{
		TransactionHash:  *event.TransactionHash,
		TransactionIndex: event.TransactionIndex,
		EventIndex:       event.EventIndex,
	}
	if !s.sentCache.ShouldSend(blockNum, &sentEvent, &finalityStatus) {
		return nil
	}
	s.sentCache.Put(blockNum, &sentEvent, &finalityStatus)

	emittedEvent := EmittedEvent{
		BlockNumber:      event.BlockNumber,
		BlockHash:        event.BlockHash,
		TransactionHash:  event.TransactionHash,
		TransactionIndex: event.TransactionIndex,
		EventIndex:       event.EventIndex,
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

	return sendEvent(s.conn, response, id)
}

func sendEvent(w jsonrpc.Conn, event *SubscriptionEmittedEvent, id string) error {
	return sendResponse("starknet_subscriptionEvents", w, id, event)
}
