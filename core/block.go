package core

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/bits-and-blooms/bloom/v3"
	"github.com/sourcegraph/conc"
)

type Header struct {
	// The hash of this block
	Hash *felt.Felt
	// The hash of this block’s parent
	ParentHash *felt.Felt
	// The number (height) of this block
	Number uint64
	// The state commitment after this block
	GlobalStateRoot *felt.Felt
	// The Starknet address of the sequencer who created this block
	SequencerAddress *felt.Felt
	// The amount Transactions and Receipts stored in this block
	TransactionCount uint64
	// The amount of events stored in transaction receipts
	EventCount uint64
	// The time the sequencer created this block before executing transactions
	Timestamp uint64
	// The version of the Starknet protocol used when creating this block
	ProtocolVersion string
	// Extraneous data that might be useful for running transactions
	ExtraData *felt.Felt
	// Bloom filter on the events emitted this block
	EventsBloom *bloom.BloomFilter
	// Amount of ETH charged per Gas spent
	GasPrice *felt.Felt
	// Sequencer signatures
	Signatures [][]*felt.Felt
}

type Block struct {
	*Header
	Transactions []Transaction
	Receipts     []*TransactionReceipt
}

type blockHashMetaInfo struct {
	First07Block             uint64     // First block that uses the post-0.7.0 block hash algorithm
	UnverifiableRange        []uint64   // Range of blocks that are not verifiable
	FallBackSequencerAddress *felt.Felt // The sequencer address to use for blocks that do not have one
}

func NetworkBlockHashMetaInfo(network utils.Network) *blockHashMetaInfo {
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

type BlockCommitments struct {
	TransactionCommitment *felt.Felt
	EventCommitment       *felt.Felt
}

// VerifyBlockHash verifies the block hash. Due to bugs in Starknet alpha, not all blocks have
// verifiable hashes.
func VerifyBlockHash(b *Block, network utils.Network) (*BlockCommitments, error) {
	if len(b.Transactions) != len(b.Receipts) {
		return nil, fmt.Errorf("len of transactions: %v do not match len of receipts: %v",
			len(b.Transactions), len(b.Receipts))
	}

	for i, tx := range b.Transactions {
		if !tx.Hash().Equal(b.Receipts[i].TransactionHash) {
			return nil, fmt.Errorf(
				"transaction hash (%v) at index: %v does not match receipt's hash (%v)",
				tx.Hash().String(), i, b.Receipts[i].TransactionHash)
		}
	}

	if err := VerifyTransactions(b.Transactions, network, b.ProtocolVersion); err != nil {
		return nil, err
	}

	metaInfo := NetworkBlockHashMetaInfo(network)
	unverifiableRange := metaInfo.UnverifiableRange
	for _, fallbackSeq := range []*felt.Felt{&felt.Zero, metaInfo.FallBackSequencerAddress} {
		var overrideSeq *felt.Felt
		if b.SequencerAddress == nil {
			overrideSeq = fallbackSeq
		}

		hash, commitments, err := blockHash(b, network, overrideSeq)
		if err != nil {
			return nil, err
		}

		if hash.Equal(b.Hash) {
			return commitments, nil
		} else if unverifiableRange != nil {
			// Check if the block number is in the unverifiable range
			if b.Number >= unverifiableRange[0] && b.Number <= unverifiableRange[1] {
				// If so, return success
				return commitments, nil
			}
		}
	}
	return nil, errors.New("can not verify hash in block header")
}

// blockHash computes the block hash, with option to override sequence address
func blockHash(b *Block, network utils.Network, overrideSeqAddr *felt.Felt) (*felt.Felt, *BlockCommitments, error) {
	metaInfo := NetworkBlockHashMetaInfo(network)

	if b.Number < metaInfo.First07Block {
		return pre07Hash(b, network.ChainID())
	}
	return post07Hash(b, overrideSeqAddr)
}

// pre07Hash computes the block hash for blocks generated before Cairo 0.7.0
func pre07Hash(b *Block, chain *felt.Felt) (*felt.Felt, *BlockCommitments, error) {
	txCommitment, err := transactionCommitment(b.Transactions, b.Header.ProtocolVersion)
	if err != nil {
		return nil, nil, err
	}

	return crypto.PedersenArray(
		new(felt.Felt).SetUint64(b.Number), // block number
		b.GlobalStateRoot,                  // global state root
		&felt.Zero,                         // reserved: sequencer address
		&felt.Zero,                         // reserved: block timestamp
		new(felt.Felt).SetUint64(b.TransactionCount), // number of transactions
		txCommitment, // transaction commitment
		&felt.Zero,   // reserved: number of events
		&felt.Zero,   // reserved: event commitment
		&felt.Zero,   // reserved: protocol version
		&felt.Zero,   // reserved: extra data
		chain,        // extra data: chain id
		b.ParentHash, // parent hash
	), &BlockCommitments{TransactionCommitment: txCommitment}, nil
}

// post07Hash computes the block hash for blocks generated after Cairo 0.7.0
func post07Hash(b *Block, overrideSeqAddr *felt.Felt) (*felt.Felt, *BlockCommitments, error) {
	seqAddr := b.SequencerAddress
	if overrideSeqAddr != nil {
		seqAddr = overrideSeqAddr
	}

	wg := conc.NewWaitGroup()
	var txCommitment, eCommitment *felt.Felt
	var tErr, eErr error

	wg.Go(func() {
		txCommitment, tErr = transactionCommitment(b.Transactions, b.Header.ProtocolVersion)
	})
	wg.Go(func() {
		eCommitment, eErr = eventCommitment(b.Receipts)
	})
	wg.Wait()

	if tErr != nil {
		return nil, nil, tErr
	}
	if eErr != nil {
		return nil, nil, eErr
	}

	// Unlike the pre07Hash computation, we exclude the chain
	// id and replace the zero felt with the actual values for:
	// - sequencer address
	// - block timestamp
	// - number of events
	// - event commitment
	return crypto.PedersenArray(
		new(felt.Felt).SetUint64(b.Number),           // block number
		b.GlobalStateRoot,                            // global state root
		seqAddr,                                      // sequencer address
		new(felt.Felt).SetUint64(b.Timestamp),        // block timestamp
		new(felt.Felt).SetUint64(b.TransactionCount), // number of transactions
		txCommitment,                                 // transaction commitment
		new(felt.Felt).SetUint64(b.EventCount),       // number of events
		eCommitment,                                  // event commitment
		&felt.Zero,                                   // reserved: protocol version
		&felt.Zero,                                   // reserved: extra data
		b.ParentHash,                                 // parent block hash
	), &BlockCommitments{TransactionCommitment: txCommitment, EventCommitment: eCommitment}, nil
}

func MarshalBlockNumber(blockNumber uint64) []byte {
	const blockNumberSize = 8

	numBytes := make([]byte, blockNumberSize)
	binary.BigEndian.PutUint64(numBytes, blockNumber)

	return numBytes
}
