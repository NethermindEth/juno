package core

import (
	"fmt"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
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
	Timestamp *felt.Felt
	// The number of transactions in a block
	TransactionCount *felt.Felt
	// A commitment to the transactions included in the block
	TransactionCommitment *felt.Felt
	// The number of events
	EventCount *felt.Felt
	// A commitment to the events produced in this block
	EventCommitment *felt.Felt
	// The version of the StarkNet protocol used when creating this block
	ProtocolVersion *felt.Felt
	// Extraneous data that might be useful for running transactions
	ExtraData *felt.Felt
}

type blockHashMetaInfo struct {
	First07Block             uint64     // First block that uses the post-0.7.0 block hash algorithm
	UnverifiableRange        []uint64   // Range of blocks that are not verifiable
	FallBackSequencerAddress *felt.Felt // The sequencer address to use for blocks that do not have one
}

type UnverifiableBlockError struct {
	blockNumber uint64
}

func (e *UnverifiableBlockError) Error() string {
	return fmt.Sprintf("block is unverifiable: %d", e.blockNumber)
}

func getBlockHashMetaInfo(network utils.Network) *blockHashMetaInfo {
	switch network {
	case utils.MAINNET:
		fallBackSequencerAddress, _ := new(felt.Felt).SetString("0x021f4b90b0377c82bf330b7b5295820769e72d79d8acd0effa0ebde6e9988bc5")
		return &blockHashMetaInfo{
			First07Block:             883,
			FallBackSequencerAddress: fallBackSequencerAddress,
		}
	case utils.GOERLI:
		fallBackSequencerAddress, _ := new(felt.Felt).SetString("0x046a89ae102987331d369645031b49c27738ed096f2789c24449966da4c6de6b")
		return &blockHashMetaInfo{
			First07Block:             47028,
			UnverifiableRange:        []uint64{119802, 148428},
			FallBackSequencerAddress: fallBackSequencerAddress,
		}
	case utils.GOERLI2:
		fallBackSequencerAddress, _ := new(felt.Felt).SetString("0x046a89ae102987331d369645031b49c27738ed096f2789c24449966da4c6de6b")
		return &blockHashMetaInfo{
			First07Block:             0,
			FallBackSequencerAddress: fallBackSequencerAddress,
		}
	case utils.INTEGRATION:
		fallBackSequencerAddress, _ := new(felt.Felt).SetString("0x046a89ae102987331d369645031b49c27738ed096f2789c24449966da4c6de6b")
		return &blockHashMetaInfo{
			First07Block:             110511,
			UnverifiableRange:        []uint64{0, 110511},
			FallBackSequencerAddress: fallBackSequencerAddress,
		}
	default:
		// This should never happen
		panic(fmt.Sprintf("unknown network: %d", network))
	}
}

// Hash computes the block hash. Due to bugs in StarkNet alpha, not all blocks have
// verifiable hashes. In that case, an [UnverifiableBlockError] is returned.
func (b *Block) Hash(network utils.Network) (*felt.Felt, error) {
	blockHashMetaInfo := getBlockHashMetaInfo(network)

	unverifiableRange := blockHashMetaInfo.UnverifiableRange
	if unverifiableRange != nil {
		// Check if the block number is in the unverifiable range
		if b.Number >= unverifiableRange[0] && b.Number <= unverifiableRange[1] {
			// If so, return unverifiable block error
			return nil, &UnverifiableBlockError{blockNumber: b.Number}
		}
	}

	if b.Number < blockHashMetaInfo.First07Block {
		return b.pre07Hash(network.ChainId()), nil
	} else if b.SequencerAddress == nil {
		b.SequencerAddress = blockHashMetaInfo.FallBackSequencerAddress
	}
	return b.post07Hash(), nil
}

// pre07Hash computes the block hash for blocks generated before Cairo 0.7.0
func (b *Block) pre07Hash(chain *felt.Felt) *felt.Felt {
	blockNumber := new(felt.Felt).SetUint64(b.Number)
	zeroFelt := new(felt.Felt)

	return crypto.PedersenArray(
		blockNumber,             // block number
		b.GlobalStateRoot,       // global state root
		zeroFelt,                // reserved: sequencer address
		zeroFelt,                // reserved: block timestamp
		b.TransactionCount,      // number of transactions
		b.TransactionCommitment, // transaction commitment
		zeroFelt,                // reserved: number of events
		zeroFelt,                // reserved: event commitment
		zeroFelt,                // reserved: protocol version
		zeroFelt,                // reserved: extra data
		chain,                   // extra data: chain id
		b.ParentHash,            // parent hash
	)
}

// post07Hash computes the block hash for blocks generated after Cairo 0.7.0
func (b *Block) post07Hash() *felt.Felt {
	blockNumber := new(felt.Felt).SetUint64(b.Number)
	zeroFelt := new(felt.Felt)

	// Unlike the pre07Hash computation, we exclude the chain
	// id and replace the zero felt with the actual values for:
	// - sequencer address
	// - block timestamp
	// - number of events
	// - event commitment
	return crypto.PedersenArray(
		blockNumber,             // block number
		b.GlobalStateRoot,       // global state root
		b.SequencerAddress,      // sequencer address
		b.Timestamp,             // block timestamp
		b.TransactionCount,      // number of transactions
		b.TransactionCommitment, // transaction commitment
		b.EventCount,            // number of events
		b.EventCommitment,       // event commitment
		zeroFelt,                // reserved: protocol version
		zeroFelt,                // reserved: extra data
		b.ParentHash,            // parent block hash
	)
}
