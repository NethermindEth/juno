package p2p

import (
	"context"
	"fmt"
	"sync"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/p2pproto"
	"github.com/NethermindEth/juno/utils"
	"github.com/hashicorp/golang-lru/simplelru"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/pkg/errors"
)

const blockSyncProto = "/core/blocks-sync/1"

const headerLRUSize = 1000

type streamProvider = func(ctx context.Context) (network.Stream, func(), error)

type BlockSyncProvider struct {
	streamProvider streamProvider
	logger         utils.SimpleLogger

	converter *converter
	verifier  *verifier

	lruMutex               *sync.Mutex
	headerByBlockNumberLru *simplelru.LRU
}

func NewBlockSyncPeerManager(
	streamProvider streamProvider,
	bc *blockchain.Blockchain,
	logger utils.SimpleLogger,
) (*BlockSyncProvider, error) {
	converter := NewConverter(&blockchainClassProvider{
		blockchain: bc,
	})

	lru, err := simplelru.NewLRU(headerLRUSize, func(key interface{}, value interface{}) {})
	if err != nil {
		return nil, err
	}

	peerManager := &BlockSyncProvider{
		streamProvider: streamProvider,
		converter:      converter,
		verifier: &verifier{
			network: bc.Network(),
		},
		logger: logger,

		lruMutex:               &sync.Mutex{},
		headerByBlockNumberLru: lru,
	}

	return peerManager, nil
}

func (ip *BlockSyncProvider) getHeaderByBlockNumber(ctx context.Context, number uint64) (*p2pproto.BlockHeader, error) {
	ip.lruMutex.Lock()
	cachedHeader, ok := ip.headerByBlockNumberLru.Get(number)
	ip.lruMutex.Unlock()
	if ok {
		return cachedHeader.(*p2pproto.BlockHeader), nil
	}

	request := &p2pproto.GetBlockHeaders{
		StartBlock: &p2pproto.GetBlockHeaders_BlockNumber{
			BlockNumber: number,
		},
		Count: 1,
	}

	headerResponse, err := ip.sendBlockSyncRequest(ctx,
		&p2pproto.Request{
			Request: &p2pproto.Request_GetBlockHeaders{
				GetBlockHeaders: request,
			},
		})
	if err != nil {
		return nil, err
	}

	blockHeaders := headerResponse.GetBlockHeaders().GetHeaders()
	if blockHeaders == nil {
		return nil, fmt.Errorf("block headers is nil")
	}

	if len(blockHeaders) != 1 {
		return nil, fmt.Errorf("unexpected number of block headers. Expected: 1, Actual: %d", len(blockHeaders))
	}

	ip.lruMutex.Lock()
	defer ip.lruMutex.Unlock()
	ip.headerByBlockNumberLru.Add(number, blockHeaders[0])

	return blockHeaders[0], nil
}

func (ip *BlockSyncProvider) getBlockByHeaderRequest(
	ctx context.Context,
	headerRequest *p2pproto.GetBlockHeaders,
) (*core.Block, map[felt.Felt]core.Class, error) {
	// The core block need both header and block to build. So.. kinda cheating here as it fetch both header and body.
	headerResponse, err := ip.sendBlockSyncRequest(ctx,
		&p2pproto.Request{
			Request: &p2pproto.Request_GetBlockHeaders{
				GetBlockHeaders: headerRequest,
			},
		})
	if err != nil {
		return nil, nil, err
	}

	blockHeaders := headerResponse.GetBlockHeaders().GetHeaders()
	if len(blockHeaders) == 0 {
		return nil, nil, nil
	}

	if len(blockHeaders) != 1 {
		return nil, nil, fmt.Errorf("unexpected number of block headers. Expected: 1, Actual: %d", len(blockHeaders))
	}

	header := blockHeaders[0]

	ip.lruMutex.Lock()
	ip.headerByBlockNumberLru.Add(header.BlockNumber, header)
	ip.lruMutex.Unlock()

	response, err := ip.sendBlockSyncRequest(ctx, &p2pproto.Request{
		Request: &p2pproto.Request_GetBlockBodies{
			GetBlockBodies: &p2pproto.GetBlockBodies{
				StartBlock: header.Hash,
				Count:      1,
				SizeLimit:  1,
				Direction:  p2pproto.Direction_FORWARD,
			},
		},
	})
	if err != nil {
		return nil, nil, errors.Wrap(err, "unable to request body from peer")
	}

	bodies := response.GetBlockBodies().GetBlockBodies()
	if len(bodies) < 1 {
		return nil, nil, errors.New("unable to fetch body")
	}

	body := bodies[0]
	block, declaredClasses, err := ip.converter.protobufHeaderAndBodyToCoreBlock(header, body)
	if err != nil {
		return nil, nil, errors.Wrap(err, "unable to convert to core body")
	}

	err = ip.verifier.VerifyBlock(block)
	if err != nil {
		return nil, nil, errors.Wrap(err, "block verification failed")
	}

	for classHash, class := range declaredClasses {
		err = ip.verifier.VerifyClass(class, &classHash)
		if err != nil {
			return nil, nil, errors.Wrap(err, "class verification failed")
		}
	}

	return block, declaredClasses, nil
}

func (ip *BlockSyncProvider) sendBlockSyncRequest(ctx context.Context, request *p2pproto.Request) (*p2pproto.Response, error) {
	stream, closeFunc, err := ip.streamProvider(ctx)
	if err != nil {
		return nil, err
	}

	defer closeFunc()

	defer func(stream network.Stream) {
		err = stream.Close()
		if err != nil {
			ip.logger.Warnw("error closing stream", "error", err)
		}
	}(stream)

	err = writeCompressedProtobuf(stream, request)
	if err != nil {
		return nil, err
	}
	err = stream.CloseWrite()
	if err != nil {
		return nil, err
	}

	resp := &p2pproto.Response{}
	err = readCompressedProtobuf(stream, resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (ip *BlockSyncProvider) GetBlockByNumber(ctx context.Context, number uint64) (*core.Block, map[felt.Felt]core.Class, error) {
	request := &p2pproto.GetBlockHeaders{
		StartBlock: &p2pproto.GetBlockHeaders_BlockNumber{
			BlockNumber: number,
		},
		Count: 1,
	}

	return ip.getBlockByHeaderRequest(ctx, request)
}

func (ip *BlockSyncProvider) GetBlockByHash(ctx context.Context, hash *felt.Felt) (*core.Block, map[felt.Felt]core.Class, error) {
	request := &p2pproto.GetBlockHeaders{
		StartBlock: &p2pproto.GetBlockHeaders_BlockHash{
			BlockHash: feltToFieldElement(hash),
		},
		Count: 1,
	}

	return ip.getBlockByHeaderRequest(ctx, request)
}

func (ip *BlockSyncProvider) GetStateUpdate(ctx context.Context, number uint64) (*core.StateUpdate, error) {
	// Ideally, it should pass the block number. but we'll just wing it here for now.
	header, err := ip.getHeaderByBlockNumber(ctx, number)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to determine blockhash for block number %d", number)
	}

	response, err := ip.sendBlockSyncRequest(ctx,
		&p2pproto.Request{
			Request: &p2pproto.Request_GetStateDiffs{
				GetStateDiffs: &p2pproto.GetStateDiffs{
					StartBlock: header.Hash,
					Count:      1,
					SizeLimit:  1,
					Direction:  0,
				},
			},
		})
	if err != nil {
		return nil, errors.Wrap(err, "unable to fetch state diff")
	}

	stateUpdates := response.GetStateDiffs().GetBlockStateUpdates()
	if len(stateUpdates) != 1 {
		return nil, errors.New("unable tow fetch state diff. Empty response.")
	}

	coreStateUpdate := protobufStateUpdateToCoreStateUpdate(stateUpdates[0])

	// Verification need these
	oldRoot := &felt.Zero // TODO: genesis have root maybe?
	if number != 0 {
		// Ah.. great.
		parentHeader, err := ip.getHeaderByBlockNumber(ctx, number-1)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to determine parent block state for block number %d", number)
		}

		oldRoot = fieldElementToFelt(parentHeader.GlobalStateRoot)
	}
	coreStateUpdate.BlockHash = fieldElementToFelt(header.Hash)
	coreStateUpdate.NewRoot = fieldElementToFelt(header.GlobalStateRoot)
	coreStateUpdate.OldRoot = oldRoot

	return coreStateUpdate, nil
}
