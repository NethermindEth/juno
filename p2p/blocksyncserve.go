package p2p

import (
	"fmt"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/p2p/grpcclient"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/pkg/errors"
	"reflect"
)

type blockSyncServer struct {
	blockchain *blockchain.Blockchain
	converter  converter
}

func (s *blockSyncServer) HandleGetBlockHeader(request *grpcclient.GetBlockHeaders) (*grpcclient.BlockHeaders, error) {
	var err error
	var startblock *core.Block
	if hash, ok := request.StartBlock.(*grpcclient.GetBlockHeaders_BlockHash); ok {
		felt := fieldElementToFelt(hash.BlockHash)
		startblock, err = s.blockchain.BlockByHash(felt)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to get block by hash %s", felt)
		}
	} else if blocknum, ok := request.StartBlock.(*grpcclient.GetBlockHeaders_BlockNumber); ok {
		startblock, err = s.blockchain.BlockByNumber(blocknum.BlockNumber)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to get block by number %d", blocknum.BlockNumber)
		}
	} else {
		return nil, fmt.Errorf("unsupported startblock type %s", reflect.TypeOf(request.StartBlock))
	}

	// TODO: request.sizelimit
	results := make([]*grpcclient.BlockHeader, 0)
	for i := 0; i < int(request.Count); i++ {
		protoheader, err := s.converter.coreBlockToProtobufHeader(startblock)
		if err != nil {
			return nil, errors.Wrap(err, "unable to convert block to protobuff block header")
		}

		results = append(results, protoheader)

		if i+1 < int(request.Count) {
			// TODO: how notfound is represented and what if its null
			if request.Direction == grpcclient.Direction_FORWARD {
				startblock, err = s.blockchain.BlockByNumber(startblock.Number + 1)
				if err != nil {
					return nil, errors.Wrapf(err, "unable to get next block %d", startblock.Number+1)
				}
			} else {
				startblock, err = s.blockchain.BlockByNumber(startblock.Number - 1)
				if err != nil {
					return nil, errors.Wrapf(err, "unable to get next block %d", startblock.Number-1)
				}
			}
		}
	}

	return &grpcclient.BlockHeaders{
		Headers: results,
	}, nil
}

func (s *blockSyncServer) HandleGetBlockBodies(request *grpcclient.GetBlockBodies) (*grpcclient.BlockBodies, error) {
	var err error
	var startblock *core.Block
	felt := fieldElementToFelt(request.StartBlock)
	startblock, err = s.blockchain.BlockByHash(felt)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get block by hash %s", felt)
	}

	// TODO: request.sizelimit
	results := make([]*grpcclient.BlockBody, 0)
	for i := 0; i < int(request.Count); i++ {
		block, err := s.converter.coreBlockToProtobufBody(startblock)
		if err != nil {
			return nil, errors.Wrap(err, "unable to convert block to protobuf")
		}
		results = append(results, block)

		if i+1 < int(request.Count) {
			// TODO: how notfound is represented and what if its null or the number overflow
			if request.Direction == grpcclient.Direction_FORWARD {
				startblock, err = s.blockchain.BlockByNumber(startblock.Number + 1)
				if err != nil {
					return nil, errors.Wrapf(err, "unable to get next block %d", startblock.Number+1)
				}
			} else {
				startblock, err = s.blockchain.BlockByNumber(startblock.Number - 1)
				if err != nil {
					return nil, errors.Wrapf(err, "unable to get next block %d", startblock.Number-1)
				}
			}
		}
	}

	return &grpcclient.BlockBodies{
		BlockBodies: results,
	}, nil
}

func (s *blockSyncServer) HandleGetStateDiff(request *grpcclient.GetStateDiffs) (*grpcclient.StateDiffs, error) {
	felt := fieldElementToFelt(request.StartBlock)
	blockheader, err := s.blockchain.BlockHeaderByHash(felt)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get block number by hash %s", felt)
	}

	blocknumber := blockheader.Number

	// TODO: request.sizelimit
	results := make([]*grpcclient.StateDiffs_BlockStateUpdateWithHash, 0)
	for i := 0; i < int(request.Count); i++ {
		diff, err := s.blockchain.StateUpdateByNumber(blocknumber)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to get block by hash %s", felt)
		}

		results = append(results, coreStateUpdateToProtobufStateUpdate(diff))

		if i+1 < int(request.Count) {
			// TODO: overflow
			if request.Direction == grpcclient.Direction_FORWARD {
				blocknumber++
			} else {
				blocknumber--
			}
		}
	}

	return &grpcclient.StateDiffs{
		BlockStateUpdates: results,
	}, nil
}

func (s *blockSyncServer) HandleStatus(request *grpcclient.Status) (*grpcclient.Status, error) {
	headBlock, err := s.blockchain.Head()
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get chain head")
	}

	return &grpcclient.Status{
		Height:  headBlock.Number,
		Hash:    feltToFieldElement(headBlock.Hash),
		ChainId: request.ChainId, // TODO: thers probably a special calculation for hash per chain. I'm just filling things here
	}, nil
}

func (s *blockSyncServer) HandleBlockSyncRequest(request *grpcclient.Request) (*grpcclient.Response, error) {
	if request.GetStatus() != nil {
		status, err := s.HandleStatus(request.GetStatus())
		if err != nil {
			return nil, errors.Wrap(err, "error handling status request")
		}

		return &grpcclient.Response{
			Response: &grpcclient.Response_Status{
				Status: status,
			},
		}, nil
	}

	if request.GetGetBlockHeaders() != nil {
		headers, err := s.HandleGetBlockHeader(request.GetGetBlockHeaders())
		if err != nil {
			return nil, errors.Wrap(err, "error handling et block headers request")
		}

		return &grpcclient.Response{
			Response: &grpcclient.Response_BlockHeaders{
				BlockHeaders: headers,
			},
		}, nil
	}

	if request.GetGetBlockBodies() != nil {
		bodies, err := s.HandleGetBlockBodies(request.GetGetBlockBodies())
		if err != nil {
			return nil, errors.Wrap(err, "error handling get block bodies request")
		}

		return &grpcclient.Response{
			Response: &grpcclient.Response_BlockBodies{
				BlockBodies: bodies,
			},
		}, nil
	}

	if request.GetGetStateDiffs() != nil {
		statediffs, err := s.HandleGetStateDiff(request.GetGetStateDiffs())
		if err != nil {
			return nil, errors.Wrap(err, "error handling status request")
		}

		return &grpcclient.Response{
			Response: &grpcclient.Response_StateDiffs{
				StateDiffs: statediffs,
			},
		}, nil
	}

	return nil, fmt.Errorf("unsupported request %s", reflect.TypeOf(request.Request))
}

func (s *blockSyncServer) handleBlockSyncStream(stream network.Stream) {
	err := s.doHandleBlockSyncStream(stream)
	if err != nil {
		fmt.Printf("error handling block sync %s\n", err)
	}
	err = stream.Close()
	if err != nil {
		fmt.Printf("error closing steram %s\n", err)
	}
}

func (s *blockSyncServer) doHandleBlockSyncStream(stream network.Stream) error {
	fmt.Printf("Handling block sync\n")

	msg := grpcclient.Request{}
	err := readCompressedProtobuf(stream, &msg)
	if err != nil {
		return err
	}

	fmt.Printf("Handling block sync type %s\n", reflect.TypeOf(msg.Request))
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
