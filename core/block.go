package core

import (
	"encoding/binary"
	"errors"
	"fmt"
	"slices"

	"github.com/Masterminds/semver/v3"
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
	// Bloom filter on the events emitted this block
	EventsBloom *bloom.BloomFilter
	// Amount of WEI charged per Gas spent
	GasPrice *felt.Felt
	// Sequencer signatures
	Signatures [][]*felt.Felt
	// Amount of STRK charged per Gas spent
	GasPriceSTRK *felt.Felt
	// The mode of the L1 data availability
	L1DAMode L1DAMode
	// The gas price for L1 data availability
	L1DataGasPrice *GasPrice
}

type L1DAMode uint

const (
	Calldata L1DAMode = iota
	Blob
)

type GasPrice struct {
	PriceInWei *felt.Felt
	PriceInFri *felt.Felt
}

type Block struct {
	*Header
	Transactions []Transaction
	Receipts     []*TransactionReceipt
}

type BlockCommitments struct {
	TransactionCommitment *felt.Felt
	EventCommitment       *felt.Felt
	ReceiptCommitment     *felt.Felt
	StateDiffCommitment   *felt.Felt
}

// VerifyBlockHash verifies the block hash. Due to bugs in Starknet alpha, not all blocks have
// verifiable hashes.
func VerifyBlockHash(b *Block, network *utils.Network, stateDiff *StateDiff) (*BlockCommitments, error) {
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

	metaInfo := network.BlockHashMetaInfo
	unverifiableRange := metaInfo.UnverifiableRange

	skipVerification := unverifiableRange != nil && b.Number >= unverifiableRange[0] && b.Number <= unverifiableRange[1] //nolint:gocritic
	// todo should we still keep it after p2p ?
	if !skipVerification {
		if err := VerifyTransactions(b.Transactions, network, b.ProtocolVersion); err != nil {
			return nil, err
		}
	}

	fallbackSeqAddresses := []*felt.Felt{&felt.Zero}
	if metaInfo.FallBackSequencerAddress != nil {
		fallbackSeqAddresses = append(fallbackSeqAddresses, metaInfo.FallBackSequencerAddress)
	}

	for _, fallbackSeq := range fallbackSeqAddresses {
		var overrideSeq *felt.Felt
		if b.SequencerAddress == nil {
			overrideSeq = fallbackSeq
		}

		hash, commitments, err := blockHash(b, stateDiff, network, overrideSeq)
		if err != nil {
			return nil, err
		}

		if hash.Equal(b.Hash) {
			return commitments, nil
		} else if skipVerification {
			// Check if the block number is in the unverifiable range
			// If so, return success
			return commitments, nil
		}
	}

	return nil, errors.New("can not verify hash in block header")
}

// blockHash computes the block hash, with option to override sequence address
func blockHash(b *Block, stateDiff *StateDiff, network *utils.Network, overrideSeqAddr *felt.Felt) (*felt.Felt, *BlockCommitments, error) {
	metaInfo := network.BlockHashMetaInfo

	blockVer, err := ParseBlockVersion(b.ProtocolVersion)
	if err != nil {
		return nil, nil, err
	}
	v0_13_2 := semver.MustParse("0.13.2")

	if blockVer.LessThan(Ver0_13_2) {
		if b.Number < metaInfo.First07Block {
			return pre07Hash(b, network.L2ChainIDFelt())
		}
		return post07Hash(b, overrideSeqAddr)
	}

	return Post0132Hash(b, stateDiff)
}

// pre07Hash computes the block hash for blocks generated before Cairo 0.7.0
func pre07Hash(b *Block, chain *felt.Felt) (*felt.Felt, *BlockCommitments, error) {
	txCommitment, err := transactionCommitmentPedersen(b.Transactions, b.Header.ProtocolVersion)
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

func Post0132Hash(b *Block, stateDiff *StateDiff) (*felt.Felt, *BlockCommitments, error) {
	var txCommitment, eCommitment, rCommitment, sdCommitment *felt.Felt
	var sdLength uint64
	var tErr, eErr, rErr error

	wg := conc.NewWaitGroup()
	wg.Go(func() {
		txCommitment, tErr = transactionCommitmentPoseidon(b.Transactions)
	})
	wg.Go(func() {
		eCommitment, eErr = eventCommitmentPoseidon(b.Receipts)
	})
	wg.Go(func() {
		rCommitment, rErr = receiptCommitment(b.Receipts)
	})
	wg.Go(func() {
		sdLength = stateDiff.Length()
		sdCommitment = stateDiff.Hash()
	})
	wg.Wait()

	if tErr != nil {
		return nil, nil, tErr
	}
	if eErr != nil {
		return nil, nil, eErr
	}
	if rErr != nil {
		return nil, nil, rErr
	}

	concatCounts := concatCounts(b.TransactionCount, b.EventCount, sdLength, b.L1DAMode)

	return crypto.PoseidonArray(
			new(felt.Felt).SetBytes([]byte("STARKNET_BLOCK_HASH0")),
			new(felt.Felt).SetUint64(b.Number),    // block number
			b.GlobalStateRoot,                     // global state root
			b.SequencerAddress,                    // sequencer address
			new(felt.Felt).SetUint64(b.Timestamp), // block timestamp
			concatCounts,
			sdCommitment,
			txCommitment,   // transaction commitment
			eCommitment,    // event commitment
			rCommitment,    // receipt commitment
			b.GasPrice,     // gas price in wei
			b.GasPriceSTRK, // gas price in fri
			b.L1DataGasPrice.PriceInWei,
			b.L1DataGasPrice.PriceInFri,
			new(felt.Felt).SetBytes([]byte(b.ProtocolVersion)),
			&felt.Zero,   // reserved: extra data
			b.ParentHash, // parent block hash
		), &BlockCommitments{
			TransactionCommitment: txCommitment,
			EventCommitment:       eCommitment,
			ReceiptCommitment:     rCommitment,
			StateDiffCommitment:   sdCommitment,
		}, nil
}

// post07Hash computes the block hash for blocks generated after Cairo 0.7.0
func post07Hash(b *Block, overrideSeqAddr *felt.Felt) (*felt.Felt, *BlockCommitments, error) {
	seqAddr := b.SequencerAddress
	if overrideSeqAddr != nil {
		seqAddr = overrideSeqAddr
	}

	wg := conc.NewWaitGroup()
	var txCommitment, eCommitment, rCommitment *felt.Felt
	var tErr, eErr, rErr error

	wg.Go(func() {
		txCommitment, tErr = transactionCommitmentPedersen(b.Transactions, b.Header.ProtocolVersion)
	})
	wg.Go(func() {
		eCommitment, eErr = eventCommitmentPedersen(b.Receipts)
	})
	wg.Go(func() {
		// even though rCommitment is not required for pre 0.13.2 hash
		// we need to calculate it for BlockCommitments that will be stored in db
		rCommitment, rErr = receiptCommitment(b.Receipts)
	})
	wg.Wait()

	if tErr != nil {
		return nil, nil, tErr
	}
	if eErr != nil {
		return nil, nil, eErr
	}
	if rErr != nil {
		return nil, nil, rErr
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
	), &BlockCommitments{TransactionCommitment: txCommitment, EventCommitment: eCommitment, ReceiptCommitment: rCommitment}, nil
}

func MarshalBlockNumber(blockNumber uint64) []byte {
	const blockNumberSize = 8

	numBytes := make([]byte, blockNumberSize)
	binary.BigEndian.PutUint64(numBytes, blockNumber)

	return numBytes
}

func concatCounts(txCount, eventCount, stateDiffLen uint64, l1Mode L1DAMode) *felt.Felt {
	var l1DAByte byte
	if l1Mode == Blob {
		l1DAByte = 0b10000000
	}

	var txCountBytes, eventCountBytes, stateDiffLenBytes [8]byte
	binary.BigEndian.PutUint64(txCountBytes[:], txCount)
	binary.BigEndian.PutUint64(eventCountBytes[:], eventCount)
	binary.BigEndian.PutUint64(stateDiffLenBytes[:], stateDiffLen)

	zeroPadding := make([]byte, 7) //nolint:mnd

	concatBytes := slices.Concat(
		txCountBytes[:],
		eventCountBytes[:],
		stateDiffLenBytes[:],
		[]byte{l1DAByte},
		zeroPadding,
	)
	return new(felt.Felt).SetBytes(concatBytes)
}
