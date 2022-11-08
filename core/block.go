package core

import (
	"github.com/NethermindEth/juno/core/felt"
)

type Block struct {
	// The hash of this block’s parent
	ParentHash *felt.Felt
	// The number (height) of this block
	Number uint64
	// The state commitment after this block
	GlobalStateRoot *felt.Felt
	// The StarkNet address of the sequencer who created this block
	SequencerAddress *felt.Felt
	// The time the sequencer created this block before executing transactions
	Timestamp uint64
	// The number of transactions in a block
	TransactionCount uint64
	// A commitment to the transactions included in the block
	TransactionCommitment *felt.Felt
	// The number of events
	EventCount uint64
	// A commitment to the events produced in this block
	EventCommitment *felt.Felt
	// The version of the StarkNet protocol used when creating this block
	ProtocolVersion uint64
	// Extraneous data that might be useful for running transactions
	ExtraData *felt.Felt
}

func (b *Block) Hash() *felt.Felt {
	// Todo: implement pedersen hash as defined here
	// https://docs.starknet.io/documentation/develop/Blocks/header/#block_hash
	return nil
}
