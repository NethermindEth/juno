package core2p2p

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
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

func AdaptHeader(header *core.Header, commitments *core.BlockCommitments) *spec.SignedBlockHeader {
	// todo revisit
	return &spec.SignedBlockHeader{
		ParentHash:       AdaptHash(header.ParentHash),
		Number:           header.Number,
		Time:             header.Timestamp,
		SequencerAddress: AdaptAddress(header.SequencerAddress),
		Receipts:         nil, // not defined yet
		State: &spec.Patricia{
			Height: core.ContractStorageTrieHeight,
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
		ProtocolVersion: header.ProtocolVersion,
		GasPrice:        AdaptFelt(header.GasPrice),
		Signatures:      utils.Map(header.Signatures, AdaptSignature),
	}
}

func AdaptEvent(e *core.Event, txH *felt.Felt) *spec.Event {
	if e == nil {
		return nil
	}

	return &spec.Event{
		TransactionHash: AdaptHash(txH),
		FromAddress:     AdaptFelt(e.From),
		Keys:            utils.Map(e.Keys, AdaptFelt),
		Data:            utils.Map(e.Data, AdaptFelt),
	}
}
