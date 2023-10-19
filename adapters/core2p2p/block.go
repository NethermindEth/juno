package core2p2p

import (
	"time"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func AdaptBlockID(header *core.Header) *spec.BlockID {
	if header == nil {
		return nil
	}

	return &spec.BlockID{
		Number: header.Number,
		Header: AdaptHash(header.Hash),
	}
}

func AdaptSignature(sig []*felt.Felt) *spec.ConsensusSignature {
	return &spec.ConsensusSignature{
		R: AdaptFelt(sig[0]),
		S: AdaptFelt(sig[1]),
	}
}

func AdaptHeader(header *core.Header, commitments *core.BlockCommitments) *spec.BlockHeader {
	return &spec.BlockHeader{
		ParentHeader:     AdaptHash(header.ParentHash),
		Number:           header.Number,
		Time:             timestamppb.New(time.Unix(int64(header.Timestamp), 0)),
		SequencerAddress: AdaptAddress(header.SequencerAddress),
		ProofFact:        nil, // not defined yet
		Receipts:         nil, // not defined yet
		StateDiffs:       nil, // not defined yet
		State: &spec.Patricia{
			Height: 251, // fixed
			Root:   AdaptHash(header.GlobalStateRoot),
		},
		Transactions: &spec.Merkle{
			NLeaves: uint32(header.TransactionCount),
			Root:    AdaptHash(commitments.TransactionCommitment),
		},
		Events: &spec.Merkle{
			NLeaves: uint32(header.EventCount),
			Root:    AdaptHash(commitments.EventCommitment),
		},
	}
}

func AdaptEvent(e *core.Event) *spec.Event {
	if e == nil {
		return nil
	}

	return &spec.Event{
		FromAddress: AdaptFelt(e.From),
		Keys:        utils.Map(e.Keys, AdaptFelt),
		Data:        utils.Map(e.Data, AdaptFelt),
	}
}
