package core

import (
	"errors"
	"strconv"

	"github.com/NethermindEth/juno/core/crypto"
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

func getBlockHashMetaInfo(chain string) (*blockHashMetaInfo, error) {
	switch chain {
	case "SN_MAIN":
		fallBackSequencerAddress, _ := new(felt.Felt).SetString("0x021f4b90b0377c82bf330b7b5295820769e72d79d8acd0effa0ebde6e9988bc5")
		return &blockHashMetaInfo{
			First07Block:             883,
			UnverifiableRange:        nil,
			FallBackSequencerAddress: fallBackSequencerAddress,
		}, nil
	case "SN_GOERLI":
		fallBackSequencerAddress, _ := new(felt.Felt).SetString("0x046a89ae102987331d369645031b49c27738ed096f2789c24449966da4c6de6b")
		return &blockHashMetaInfo{
			First07Block:             47028,
			UnverifiableRange:        []uint64{119802, 148428},
			FallBackSequencerAddress: fallBackSequencerAddress,
		}, nil
	case "SN_GOERLI2":
		fallBackSequencerAddress, _ := new(felt.Felt).SetString("0x046a89ae102987331d369645031b49c27738ed096f2789c24449966da4c6de6b")
		return &blockHashMetaInfo{
			First07Block:             0,
			UnverifiableRange:        nil,
			FallBackSequencerAddress: fallBackSequencerAddress,
		}, nil
	case "SN_INTEGRATION":
		fallBackSequencerAddress, _ := new(felt.Felt).SetString("0x046a89ae102987331d369645031b49c27738ed096f2789c24449966da4c6de6b")
		return &blockHashMetaInfo{
			First07Block:             110511,
			UnverifiableRange:        []uint64{0, 110511},
			FallBackSequencerAddress: fallBackSequencerAddress,
		}, nil
	default:
		return nil, errors.New("unknown chain: " + chain)
	}
}

// Block hash computation according to https://docs.starknet.io/documentation/develop/Blocks/header/#block_hash
func (b *Block) Hash(chain string) (*felt.Felt, error) {
	blockHashMetaInfo, err := getBlockHashMetaInfo(chain)
	if err != nil {
		return nil, err
	}

	unverifiableRange := blockHashMetaInfo.UnverifiableRange
	if unverifiableRange != nil {
		// Check if the block number is in the unverifiable range
		if b.Number >= unverifiableRange[0] && b.Number <= unverifiableRange[1] {
			// If so, return unverifiable block error
			return nil, errors.New("block is unverifiable" + strconv.FormatUint(b.Number, 10))
		}
	}

	if b.Number < blockHashMetaInfo.First07Block {
		return b.pre07Hash(chain)
	} else if b.SequencerAddress == nil {
		b.SequencerAddress = blockHashMetaInfo.FallBackSequencerAddress
	}
	return b.post07Hash()
}

// Computes the block hash for blocks generated before Cairo 0.7.0
//
// The major difference between the pre-0.7.0 and post-0.7.0 block hashes is that
// the pre-0.7.0 block hash includes the chain id in the hash computation.
//
// Also, for these blocks, we use zeroes for:
// - sequencer address
// - block timestamp
// - number of events
// - event commitment
func (b *Block) pre07Hash(chain string) (*felt.Felt, error) {
	blockNumber := new(felt.Felt).SetUint64(b.Number)

	zeroFelt := new(felt.Felt)

	chainId := new(felt.Felt).SetBytes([]byte(chain))

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
		chainId,                 // extra data: chain id
		b.ParentHash,            // parent hash
	)
}

func (b *Block) post07Hash() (*felt.Felt, error) {
	blockNumber := new(felt.Felt).SetUint64(b.Number)

	zeroFelt := new(felt.Felt)

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
