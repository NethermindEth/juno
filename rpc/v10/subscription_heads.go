package rpcv10

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
// https://github.com/starkware-libs/starknet-specs/blob/785257f27cdc4ea0ca3b62a21b0f7bf51000f9b1/api/starknet_ws_api.json#L10
//
//nolint:lll,nolintlint // url exceeds line limit, nolintlint because conflicting line limit with other lint rules
func (h *Handler) SubscribeNewHeads(
	ctx context.Context,
	blockID *SubscriptionBlockID,
) (SubscriptionID, *jsonrpc.Error) {
	wsConn, ok := jsonrpc.ConnFromContext(ctx)
	if !ok {
		return "", jsonrpc.Err(jsonrpc.MethodNotFound, nil)
	}

	startBlock, latestBlock, rpcErr := h.resolveBlockRange(blockID)
	if rpcErr != nil {
		return "", rpcErr
	}

	return h.subscribe(wsConn, newHeadsSubscriber(h, wsConn, startBlock, latestBlock))
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
	head *core.Block,
) error {
	commitments, err := s.handler.bcReader.BlockCommitmentsByNumber(head.Number)
	if err != nil {
		return err
	}

	adaptedHeader := AdaptBlockHeader(head.Header, commitments)
	return sendHeader(s.conn, &adaptedHeader, id)
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

		commitments, err := s.handler.bcReader.BlockCommitmentsByNumber(currentBlock)
		if err != nil {
			return err
		}

		currentHeader, err := s.handler.bcReader.BlockHeaderByNumber(currentBlock)
		if err != nil {
			return err
		}

		adaptedHeader := AdaptBlockHeader(currentHeader, commitments)
		if err = sendHeader(s.conn, &adaptedHeader, id); err != nil {
			return err
		}
	}
	return nil
}

func sendHeader(wsConn jsonrpc.Conn, header *BlockHeader, id string) error {
	return sendResponse("starknet_subscriptionNewHeads", wsConn, id, header)
}
