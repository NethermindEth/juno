package core

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
)

var ErrCantVerifyBlockHash = errors.New("can not verify hash in block header")

type Header struct {
	// The hash of this block
	Hash *felt.Felt
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
	// The version of the StarkNet protocol used when creating this block
	ProtocolVersion string
	// Extraneous data that might be useful for running transactions
	ExtraData *felt.Felt
}

type Block struct {
	Header
	Transactions []Transaction
	Receipts     []*TransactionReceipt
}

type blockHashMetaInfo struct {
	First07Block             uint64     // First block that uses the post-0.7.0 block hash algorithm
	UnverifiableRange        []uint64   // Range of blocks that are not verifiable
	FallBackSequencerAddress *felt.Felt // The sequencer address to use for blocks that do not have one
}

func getBlockHashMetaInfo(network utils.Network) *blockHashMetaInfo {
	fallBackSequencerAddress, err := new(felt.Felt).SetString(
		"0x046a89ae102987331d369645031b49c27738ed096f2789c24449966da4c6de6b")
	if err != nil {
		panic(fmt.Sprintf("Error while creating FallBackSequencerAddress %s", err))
	}

	switch network {
	case utils.MAINNET:
		fallBackSequencerAddress, err = new(felt.Felt).SetString(
			"0x021f4b90b0377c82bf330b7b5295820769e72d79d8acd0effa0ebde6e9988bc5")
		if err != nil {
			panic(fmt.Sprintf("Error while creating FallBackSequencerAddress %s", err))
		}
		return &blockHashMetaInfo{
			First07Block:             833,
			FallBackSequencerAddress: fallBackSequencerAddress,
		}
	case utils.GOERLI:
		return &blockHashMetaInfo{
			First07Block:             47028,
			UnverifiableRange:        []uint64{119802, 148428},
			FallBackSequencerAddress: fallBackSequencerAddress,
		}
	case utils.GOERLI2:
		return &blockHashMetaInfo{
			First07Block:             0,
			FallBackSequencerAddress: fallBackSequencerAddress,
		}
	case utils.INTEGRATION:
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

// VerifyBlockHash verifies the block hash. Due to bugs in StarkNet alpha, not all blocks have
// verifiable hashes.
func VerifyBlockHash(b *Block, network utils.Network) error {
	metaInfo := getBlockHashMetaInfo(network)

	unverifiableRange := metaInfo.UnverifiableRange
	if unverifiableRange != nil {
		// Check if the block number is in the unverifiable range
		if b.Number >= unverifiableRange[0] && b.Number <= unverifiableRange[1] {
			// If so, return success
			return nil
		}
	}

	for _, fallbackSeq := range []*felt.Felt{&felt.Zero, metaInfo.FallBackSequencerAddress} {
		var overrideSeq *felt.Felt
		if b.SequencerAddress == nil {
			overrideSeq = fallbackSeq
		}

		if hash, err := blockHash(b, network, overrideSeq); err != nil {
			return err
		} else if hash.Equal(b.Hash) {
			return nil
		}
	}
	return ErrCantVerifyBlockHash
}

// BlockHash computes the block hash
func BlockHash(b *Block, network utils.Network) (*felt.Felt, error) {
	return blockHash(b, network, nil)
}

// blockHash computes the block hash, with option to override sequence address
func blockHash(b *Block, network utils.Network, overrideSeqAddr *felt.Felt) (*felt.Felt, error) {
	metaInfo := getBlockHashMetaInfo(network)

	if b.Number < metaInfo.First07Block {
		return pre07Hash(b, network.ChainId())
	}
	return post07Hash(b, overrideSeqAddr)
}

// pre07Hash computes the block hash for blocks generated before Cairo 0.7.0
func pre07Hash(b *Block, chain *felt.Felt) (*felt.Felt, error) {
	blockNumber := new(felt.Felt).SetUint64(b.Number)
	transactionCount := new(felt.Felt).SetUint64(uint64(len(b.Transactions)))
	transactionCommitment, err := TransactionCommitment(b.Transactions)
	if err != nil {
		return nil, err
	}

	return crypto.PedersenArray(
		blockNumber,           // block number
		b.GlobalStateRoot,     // global state root
		&felt.Zero,            // reserved: sequencer address
		&felt.Zero,            // reserved: block timestamp
		transactionCount,      // number of transactions
		transactionCommitment, // transaction commitment
		&felt.Zero,            // reserved: number of events
		&felt.Zero,            // reserved: event commitment
		&felt.Zero,            // reserved: protocol version
		&felt.Zero,            // reserved: extra data
		chain,                 // extra data: chain id
		b.ParentHash,          // parent hash
	), nil
}

// post07Hash computes the block hash for blocks generated after Cairo 0.7.0
func post07Hash(b *Block, overrideSeqAddr *felt.Felt) (*felt.Felt, error) {
	blockNumber := new(felt.Felt).SetUint64(b.Number)
	seqAddr := b.SequencerAddress
	if overrideSeqAddr != nil {
		seqAddr = overrideSeqAddr
	}

	transactionCount := new(felt.Felt).SetUint64(uint64(len(b.Transactions)))
	transactionCommitment, err := TransactionCommitment(b.Transactions)
	if err != nil {
		return nil, err
	}

	eventCommitment, eventCount, err := EventCommitmentAndCount(b.Receipts)
	if err != nil {
		return nil, err
	}

	// Unlike the pre07Hash computation, we exclude the chain
	// id and replace the zero felt with the actual values for:
	// - sequencer address
	// - block timestamp
	// - number of events
	// - event commitment
	return crypto.PedersenArray(
		blockNumber,                          // block number
		b.GlobalStateRoot,                    // global state root
		seqAddr,                              // sequencer address
		b.Timestamp,                          // block timestamp
		transactionCount,                     // number of transactions
		transactionCommitment,                // transaction commitment
		new(felt.Felt).SetUint64(eventCount), // number of events
		eventCommitment,                      // event commitment
		&felt.Zero,                           // reserved: protocol version
		&felt.Zero,                           // reserved: extra data
		b.ParentHash,                         // parent block hash
	), nil
}
