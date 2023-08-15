package core2p2p

import (
	"time"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func AdaptHeader(block *core.Block, commitments *core.BlockCommitments, chainID *felt.Felt) *spec.BlockHeader {
	return &spec.BlockHeader{
		ParentBlock: &spec.BlockID{
			Hash: AdaptHash(block.ParentHash),
		},
		Time:             timestamppb.New(time.Unix(int64(block.Timestamp), 0)),
		SequencerAddress: AdaptAddress(block.SequencerAddress), //todo: handle nil block.SequencerAddress
		StateDiffs:       nil,                                  //todo
		State:            AdaptHash(block.GlobalStateRoot),
		ProofFact:        nil,
		Transactions: &spec.Merkle{
			NLeaves: uint32(block.TransactionCount),
			Root:    AdaptHash(commitments.TransactionCommitment),
		},
		Events: &spec.Merkle{
			NLeaves: uint32(block.EventCount),
			Root:    AdaptHash(commitments.EventCommitment), //todo: handle nil commitments.EventCommitment
		},
		Receipts:        nil, //todo: define how to hash a receipt first?
		ProtocolVersion: 0,   //todo: what to do with this?
		ChainId:         AdaptChainID(chainID),
	}
}
