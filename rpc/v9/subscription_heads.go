package rpcv9

import (
	"context"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/sync"
)

// SubscribeNewHeads creates a WebSocket stream which will fire events when
// a new block header is added.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/c2e93098b9c2ca0423b7f4d15b201f52f22d8c36/api/starknet_ws_api.json#L10
//
//nolint:lll,nolintlint // url exceeds line limit, nolintlint because conflicting line limit with other lint rules
func (h *Handler) SubscribeNewHeads(
	ctx context.Context,
	blockID *SubscriptionBlockID,
) (SubscriptionID, *jsonrpc.Error) {
	w, ok := jsonrpc.ConnFromContext(ctx)
	if !ok {
		return "", jsonrpc.Err(jsonrpc.MethodNotFound, nil)
	}

	startBlock, latestBlock, rpcErr := h.resolveBlockRange(blockID)
	if rpcErr != nil {
		return "", rpcErr
	}

	return h.subscribe(ctx, w, newHeadsSubscriber(h, w, startBlock, latestBlock))
}

type headsSubscriberState struct {
	handler *Handler
	conn    jsonrpc.Conn
}

func newHeadsSubscriber(
	h *Handler,
	conn jsonrpc.Conn,
	startBlock,
	latestBlock uint64,
) subscriber {
	state := &headsSubscriberState{handler: h, conn: conn}

	return subscriber{
		onStart: func(ctx context.Context, id string, _ *subscription, _ any) error {
			return state.sendHistoricalHeaders(ctx, id, startBlock, latestBlock)
		},
		onReorg:   state.onReorg,
		onNewHead: state.onNewHead,
	}
}

func (s *headsSubscriberState) onReorg(
	_ context.Context,
	id string,
	_ *subscription,
	reorg *sync.ReorgBlockRange,
) error {
	return sendReorg(s.conn, reorg, id)
}

func (s *headsSubscriberState) onNewHead(
	_ context.Context,
	id string,
	_ *subscription,
	headWithBloom *core.WithBloom[*core.Block],
) error {
	return sendHeader(s.conn, headWithBloom.Value.Header, id)
}

func (s *headsSubscriberState) sendHistoricalHeaders(
	ctx context.Context,
	id string,
	startBlock,
	latestBlock uint64,
) error {
	for currentBlock := startBlock; currentBlock <= latestBlock; currentBlock++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		currentHeader, err := s.handler.bcReader.BlockHeaderByNumber(currentBlock)
		if err != nil {
			return err
		}

		if err = sendHeader(s.conn, currentHeader, id); err != nil {
			return err
		}
	}
	return nil
}

func sendHeader(w jsonrpc.Conn, header *core.Header, id string) error {
	return sendResponse("starknet_subscriptionNewHeads", w, id, AdaptBlockHeader(header))
}
