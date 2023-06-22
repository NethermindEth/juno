package p2p

import (
	"fmt"
	"reflect"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/p2p/p2pproto"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/pkg/errors"
)

type blockSyncServer struct {
	blockchain *blockchain.Blockchain
	converter  *converter

	log utils.SimpleLogger
}

func (s *blockSyncServer) HandleGetBlockHeader(request *p2pproto.GetBlockHeaders) (*p2pproto.BlockHeaders, error) {
	var err error
	var startblock *core.Block

	switch v := request.StartBlock.(type) {
	case *p2pproto.GetBlockHeaders_BlockHash:
		felt := fieldElementToFelt(v.BlockHash)
		startblock, err = s.blockchain.BlockByHash(felt)

		if err == db.ErrKeyNotFound {
			return &p2pproto.BlockHeaders{
				Headers: []*p2pproto.BlockHeader{},
			}, nil
		}

		if err != nil {
			return nil, errors.Wrapf(err, "unable to get block by hash %s", felt)
		}
	case *p2pproto.GetBlockHeaders_BlockNumber:
		startblock, err = s.blockchain.BlockByNumber(v.BlockNumber)

		if err == db.ErrKeyNotFound {
			return &p2pproto.BlockHeaders{
				Headers: []*p2pproto.BlockHeader{},
			}, nil
		}

		if err != nil {
			return nil, errors.Wrapf(err, "unable to get block by number %d", v.BlockNumber)
		}
	default:
		return nil, fmt.Errorf("unsupported startblock type %s", reflect.TypeOf(request.StartBlock))
	}

	// TODO: request.sizelimit
	results, err := mapBlockSequence(
		s.blockchain,
		int(request.Count),
		request.Direction,
		startblock,
		func(block *core.Block) (*p2pproto.BlockHeader, error) {
			return s.converter.coreBlockToProtobufHeader(block)
		})
	if err != nil {
		return nil, err
	}

	return &p2pproto.BlockHeaders{
		Headers: results,
	}, nil
}

func (s *blockSyncServer) HandleGetBlockBodies(request *p2pproto.GetBlockBodies) (*p2pproto.BlockBodies, error) {
	var err error
	var startblock *core.Block
	felt := fieldElementToFelt(request.StartBlock)
	startblock, err = s.blockchain.BlockByHash(felt)
	if err == db.ErrKeyNotFound {
		return &p2pproto.BlockBodies{
			BlockBodies: []*p2pproto.BlockBody{},
		}, nil
	}
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get block by hash %s", felt)
	}

	// TODO: request.sizelimit

	results, err := mapBlockSequence(
		s.blockchain,
		int(request.Count),
		request.Direction,
		startblock,
		func(block *core.Block) (*p2pproto.BlockBody, error) {
			return s.converter.coreBlockToProtobufBody(block)
		})

	if err != nil {
		return nil, err
	}

	return &p2pproto.BlockBodies{
		BlockBodies: results,
	}, nil
}

func mapBlockSequence[T any](
	bc *blockchain.Blockchain,
	requestCount int,
	direction p2pproto.Direction,
	startblock *core.Block,
	mapper func(block *core.Block) (T, error),
) ([]T, error) {
	results := make([]T, 0)
	for i := 0; i < requestCount; i++ {
		result, err := mapper(startblock)
		if err != nil {
			return nil, errors.Wrap(err, "unable to convert block to protobuff block header")
		}

		results = append(results, result)

		if i+1 < requestCount {
			// TODO: how notfound is represented and what if its null
			if direction == p2pproto.Direction_FORWARD {
				startblock, err = bc.BlockByNumber(startblock.Number + 1)
				if err != nil {
					return nil, errors.Wrapf(err, "unable to get next block %d", startblock.Number+1)
				}
			} else {
				startblock, err = bc.BlockByNumber(startblock.Number - 1)
				if err != nil {
					return nil, errors.Wrapf(err, "unable to get next block %d", startblock.Number-1)
				}
			}
		}
	}

	return results, nil
}

func (s *blockSyncServer) HandleGetStateDiff(request *p2pproto.GetStateDiffs) (*p2pproto.StateDiffs, error) {
	felt := fieldElementToFelt(request.StartBlock)
	blockheader, err := s.blockchain.BlockHeaderByHash(felt)
	if err == db.ErrKeyNotFound {
		return &p2pproto.StateDiffs{
			BlockStateUpdates: []*p2pproto.StateDiffs_BlockStateUpdateWithHash{},
		}, nil
	}
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get block number by hash %s", felt)
	}

	blocknumber := blockheader.Number

	// TODO: request.sizelimit
	results := make([]*p2pproto.StateDiffs_BlockStateUpdateWithHash, 0)
	for i := 0; i < int(request.Count); i++ {
		diff, err := s.blockchain.StateUpdateByNumber(blocknumber)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to get block by hash %s", felt)
		}

		results = append(results, coreStateUpdateToProtobufStateUpdate(diff))

		if i+1 < int(request.Count) {
			// TODO: overflow
			if request.Direction == p2pproto.Direction_FORWARD {
				blocknumber++
			} else {
				blocknumber--
			}
		}
	}

	return &p2pproto.StateDiffs{
		BlockStateUpdates: results,
	}, nil
}

func (s *blockSyncServer) HandleStatus(request *p2pproto.Status) (*p2pproto.Status, error) {
	headBlock, err := s.blockchain.Head()
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get chain head")
	}

	return &p2pproto.Status{
		Height:  headBlock.Number,
		Hash:    feltToFieldElement(headBlock.Hash),
		ChainId: request.ChainId, // TODO: thers probably a special calculation for hash per chain. I'm just filling things here
	}, nil
}

func (s *blockSyncServer) HandleBlockSyncRequest(request *p2pproto.Request) (*p2pproto.Response, error) {
	if request.GetStatus() != nil {
		status, err := s.HandleStatus(request.GetStatus())
		if err != nil {
			return nil, errors.Wrap(err, "error handling status request")
		}

		return &p2pproto.Response{
			Response: &p2pproto.Response_Status{
				Status: status,
			},
		}, nil
	}

	if request.GetGetBlockHeaders() != nil {
		headers, err := s.HandleGetBlockHeader(request.GetGetBlockHeaders())
		if err != nil {
			return nil, errors.Wrap(err, "error handling et block headers request")
		}

		return &p2pproto.Response{
			Response: &p2pproto.Response_BlockHeaders{
				BlockHeaders: headers,
			},
		}, nil
	}

	if request.GetGetBlockBodies() != nil {
		bodies, err := s.HandleGetBlockBodies(request.GetGetBlockBodies())
		if err != nil {
			return nil, errors.Wrap(err, "error handling get block bodies request")
		}

		return &p2pproto.Response{
			Response: &p2pproto.Response_BlockBodies{
				BlockBodies: bodies,
			},
		}, nil
	}

	if request.GetGetStateDiffs() != nil {
		statediffs, err := s.HandleGetStateDiff(request.GetGetStateDiffs())
		if err != nil {
			return nil, errors.Wrap(err, "error handling status request")
		}

		return &p2pproto.Response{
			Response: &p2pproto.Response_StateDiffs{
				StateDiffs: statediffs,
			},
		}, nil
	}

	return nil, fmt.Errorf("unsupported request %s", reflect.TypeOf(request.Request))
}

func (s *blockSyncServer) handleBlockSyncStream(stream network.Stream) {
	err := s.doHandleBlockSyncStream(stream)
	if err != nil {
		s.log.Errorw("error handling block sync", err)
	}
	err = stream.Close()
	if err != nil {
		s.log.Errorw("error closing stream", err)
	}
}

func (s *blockSyncServer) doHandleBlockSyncStream(stream network.Stream) error {
	msg := p2pproto.Request{}
	err := readCompressedProtobuf(stream, &msg)
	if err != nil {
		return err
	}

	s.log.Infow("Handling block sync", "type", reflect.TypeOf(msg.Request))
	resp, err := s.HandleBlockSyncRequest(&msg)
	if err != nil {
		return err
	}

	err = writeCompressedProtobuf(stream, resp)
	if err != nil {
		return err
	}

	return nil
}
