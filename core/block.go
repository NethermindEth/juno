package core

import (
	"encoding/binary"
	"errors"
	"fmt"
	"slices"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/bits-and-blooms/bloom/v3"
	"github.com/sourcegraph/conc"
)

type Header struct {
	// The hash of this block
	Hash *felt.Felt `cbor:"1,keyasint,omitempty"`
	// The hash of this block’s parent
	ParentHash *felt.Felt `cbor:"2,keyasint,omitempty"`
	// The number (height) of this block
	Number uint64 `cbor:"3,keyasint,omitempty"`
	// The state commitment after this block
	GlobalStateRoot *felt.Felt `cbor:"4,keyasint,omitempty"`
	// The Starknet address of the sequencer who created this block
	SequencerAddress *felt.Felt `cbor:"5,keyasint,omitempty"`
	// The amount Transactions and Receipts stored in this block
	TransactionCount uint64 `cbor:"6,keyasint,omitempty"`
	// The amount of events stored in transaction receipts
	EventCount uint64 `cbor:"7,keyasint,omitempty"`
	// The time the sequencer created this block before executing transactions
	Timestamp uint64 `cbor:"8,keyasint,omitempty"`
	// Todo(rdr): It makes more sense for Protocol version to be stored in semver.Version instead
	// The version of the Starknet protocol used when creating this block
	ProtocolVersion string `cbor:"9,keyasint,omitempty"`
	// Bloom filter on the events emitted this block
	EventsBloom *bloom.BloomFilter `cbor:"10,keyasint,omitempty"`
	// Amount of WEI charged per Gas spent on L1
	L1GasPriceETH *felt.Felt `cbor:"11,keyasint,omitempty"`
	// Amount of STRK charged per Gas spent on L2
	Signatures [][]*felt.Felt `cbor:"12,keyasint,omitempty"`
	// Amount of STRK charged per Gas spent on L1
	L1GasPriceSTRK *felt.Felt `cbor:"13,keyasint,omitempty"`
	// Amount of STRK charged per Gas spent on L2
	L1DAMode L1DAMode `cbor:"14,keyasint,omitempty"`
	// The gas price for L1 data availability
	L1DataGasPrice *GasPrice `cbor:"15,keyasint,omitempty"`
	L2GasPrice     *GasPrice `cbor:"16,keyasint,omitempty"`
}

type L1DAMode uint

const (
	Calldata L1DAMode = iota
	Blob
)

var (
	starknetBlockHash0 = new(felt.Felt).SetBytes([]byte("STARKNET_BLOCK_HASH0"))
	starknetBlockHash1 = new(felt.Felt).SetBytes([]byte("STARKNET_BLOCK_HASH1"))
	starknetGasPrices0 = new(felt.Felt).SetBytes([]byte("STARKNET_GAS_PRICES0"))
)

type GasPrice struct {
	PriceInWei *felt.Felt `cbor:"1,keyasint,omitempty"`
	PriceInFri *felt.Felt `cbor:"2,keyasint,omitempty"`
}

type Block struct {
	*Header
	Transactions []Transaction
	Receipts     []*TransactionReceipt
}

func (b *Block) L2GasConsumed() uint64 {
	l2GasConsumed := uint64(0)
	for _, t := range b.Receipts {
		l2GasConsumed += t.ExecutionResources.TotalGasConsumed.L2Gas
	}
	return l2GasConsumed
}

type BlockCommitments struct {
	TransactionCommitment *felt.Felt `cbor:"1,keyasint,omitempty"`
	EventCommitment       *felt.Felt `cbor:"2,keyasint,omitempty"`
	ReceiptCommitment     *felt.Felt `cbor:"3,keyasint,omitempty"`
	StateDiffCommitment   *felt.Felt `cbor:"4,keyasint,omitempty"`
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

		hash, commitments, err := BlockHash(b, stateDiff, network, overrideSeq)
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
func BlockHash(
	b *Block,
	stateDiff *StateDiff,
	network *utils.Network,
	overrideSeqAddr *felt.Felt,
) (felt.Felt, *BlockCommitments, error) {
	metaInfo := network.BlockHashMetaInfo

	blockVer, err := ParseBlockVersion(b.ProtocolVersion)
	if err != nil {
		return felt.Felt{}, nil, err
	}

	// if block.version >= 0.13.4
	if blockVer.GreaterThanEqual(Ver0_13_4) {
		return post0134Hash(b, stateDiff)
	}

	// if 0.13.2 <= block.version < 0.13.4
	if blockVer.GreaterThanEqual(Ver0_13_2) {
		return Post0132Hash(b, stateDiff)
	}

	// following statements applied only if block.version < 0.13.2
	if b.Number < metaInfo.First07Block {
		return pre07Hash(b, network.L2ChainIDFelt())
	}
	return post07Hash(b, overrideSeqAddr)
}

// pre07Hash computes the block hash for blocks generated before Cairo 0.7.0
func pre07Hash(b *Block, chain *felt.Felt) (felt.Felt, *BlockCommitments, error) {
	txCommitment, err := transactionCommitmentPedersen(b.Transactions, b.Header.ProtocolVersion)
	if err != nil {
		return felt.Felt{}, nil, err
	}
	return crypto.PedersenArray(
		new(felt.Felt).SetUint64(b.Number), // block number
		b.GlobalStateRoot,                  // global state root
		&felt.Zero,                         // reserved: sequencer address
		&felt.Zero,                         // reserved: block timestamp
		new(felt.Felt).SetUint64(b.TransactionCount), // number of transactions
		&txCommitment, // transaction commitment
		&felt.Zero,    // reserved: number of events
		&felt.Zero,    // reserved: event commitment
		&felt.Zero,    // reserved: protocol version
		&felt.Zero,    // reserved: extra data
		chain,         // extra data: chain id
		b.ParentHash,  // parent hash
	), &BlockCommitments{TransactionCommitment: &txCommitment}, nil
}

func post0134Hash(b *Block, stateDiff *StateDiff) (felt.Felt, *BlockCommitments, error) {
	var txCommitment, eCommitment, rCommitment, sdCommitment felt.Felt
	var sdLength uint64
	var tErr, eErr, rErr error

	wg := conc.NewWaitGroup()
	wg.Go(func() {
		txCommitment, tErr = transactionCommitmentPoseidon0134(b.Transactions)
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
		return felt.Felt{}, nil, tErr
	}
	if eErr != nil {
		return felt.Felt{}, nil, eErr
	}
	if rErr != nil {
		return felt.Felt{}, nil, rErr
	}

	concatCounts := ConcatCounts(b.TransactionCount, b.EventCount, sdLength, b.L1DAMode)

	pricesHash := gasPricesHash(
		GasPrice{
			PriceInFri: b.L1GasPriceSTRK,
			PriceInWei: b.L1GasPriceETH,
		},
		*b.L1DataGasPrice,
		*b.L2GasPrice,
	)

	return crypto.PoseidonArray(
			starknetBlockHash1,
			new(felt.Felt).SetUint64(b.Number),    // block number
			b.GlobalStateRoot,                     // global state root
			b.SequencerAddress,                    // sequencer address
			new(felt.Felt).SetUint64(b.Timestamp), // block timestamp
			&concatCounts,
			&sdCommitment,
			&txCommitment, // transaction commitment
			&eCommitment,  // event commitment
			&rCommitment,  // receipt commitment
			&pricesHash,   // gas prices hash
			new(felt.Felt).SetBytes([]byte(b.ProtocolVersion)),
			&felt.Zero,   // reserved: extra data
			b.ParentHash, // parent block hash
		), &BlockCommitments{
			TransactionCommitment: &txCommitment,
			EventCommitment:       &eCommitment,
			ReceiptCommitment:     &rCommitment,
			StateDiffCommitment:   &sdCommitment,
		}, nil
}

func Post0132Hash(b *Block, stateDiff *StateDiff) (felt.Felt, *BlockCommitments, error) {
	var txCommitment, eCommitment, rCommitment, sdCommitment felt.Felt
	var sdLength uint64
	var tErr, eErr, rErr error

	wg := conc.NewWaitGroup()
	wg.Go(func() {
		txCommitment, tErr = transactionCommitmentPoseidon0132(b.Transactions)
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
		return felt.Felt{}, nil, tErr
	}
	if eErr != nil {
		return felt.Felt{}, nil, eErr
	}
	if rErr != nil {
		return felt.Felt{}, nil, rErr
	}

	concatCounts := ConcatCounts(b.TransactionCount, b.EventCount, sdLength, b.L1DAMode)

	// These values are nil for some pre 0.13.2 blocks
	// `crypto.PoseidonArray` panics if any of the values are nil
	seqAddr := &felt.Zero
	gasPriceStrk := &felt.Zero
	l1DataGasPriceInWei := &felt.Zero
	l1DataGasPriceInFri := &felt.Zero

	if b.SequencerAddress != nil {
		seqAddr = b.SequencerAddress
	}
	if b.L1GasPriceSTRK != nil {
		gasPriceStrk = b.L1GasPriceSTRK
	}
	if b.L1DataGasPrice != nil {
		if b.L1DataGasPrice.PriceInWei != nil {
			l1DataGasPriceInWei = b.L1DataGasPrice.PriceInWei
		}
		if b.L1DataGasPrice.PriceInFri != nil {
			l1DataGasPriceInFri = b.L1DataGasPrice.PriceInFri
		}
	}

	return crypto.PoseidonArray(
			starknetBlockHash0,
			new(felt.Felt).SetUint64(b.Number),    // block number
			b.GlobalStateRoot,                     // global state root
			seqAddr,                               // sequencer address
			new(felt.Felt).SetUint64(b.Timestamp), // block timestamp
			&concatCounts,
			&sdCommitment,
			&txCommitment,   // transaction commitment
			&eCommitment,    // event commitment
			&rCommitment,    // receipt commitment
			b.L1GasPriceETH, // gas price in wei
			gasPriceStrk,    // gas price in fri
			l1DataGasPriceInWei,
			l1DataGasPriceInFri,
			new(felt.Felt).SetBytes([]byte(b.ProtocolVersion)),
			&felt.Zero,   // reserved: extra data
			b.ParentHash, // parent block hash
		), &BlockCommitments{
			TransactionCommitment: &txCommitment,
			EventCommitment:       &eCommitment,
			ReceiptCommitment:     &rCommitment,
			StateDiffCommitment:   &sdCommitment,
		}, nil
}

// post07Hash computes the block hash for blocks generated after Cairo 0.7.0
func post07Hash(b *Block, overrideSeqAddr *felt.Felt) (felt.Felt, *BlockCommitments, error) {
	seqAddr := b.SequencerAddress
	if overrideSeqAddr != nil {
		seqAddr = overrideSeqAddr
	}

	wg := conc.NewWaitGroup()
	var txCommitment, eCommitment, rCommitment felt.Felt
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
		return felt.Felt{}, nil, tErr
	}
	if eErr != nil {
		return felt.Felt{}, nil, eErr
	}
	if rErr != nil {
		return felt.Felt{}, nil, rErr
	}

	// Unlike the pre07Hash computation, we exclude the chain
	// id and replace the zero felt with the actual values for:
	// - sequencer address
	// - block timestamp
	// - number of events
	// - event commitment
	return crypto.PedersenArray(
			felt.NewFromUint64[felt.Felt](b.Number), // block number
			b.GlobalStateRoot,                       // global state root
			seqAddr,                                 // sequencer address
			felt.NewFromUint64[felt.Felt](b.Timestamp),        // block timestamp
			felt.NewFromUint64[felt.Felt](b.TransactionCount), // number of transactions
			&txCommitment, // transaction commitment
			felt.NewFromUint64[felt.Felt](b.EventCount), // number of events
			&eCommitment, // event commitment
			&felt.Zero,   // reserved: protocol version
			&felt.Zero,   // reserved: extra data
			b.ParentHash, // parent block hash
		), &BlockCommitments{
			TransactionCommitment: &txCommitment,
			EventCommitment:       &eCommitment,
			ReceiptCommitment:     &rCommitment,
		}, nil
}

func MarshalBlockNumber(blockNumber uint64) []byte {
	const blockNumberSize = 8

	numBytes := make([]byte, blockNumberSize)
	binary.BigEndian.PutUint64(numBytes, blockNumber)

	return numBytes
}

func UnmarshalBlockNumber(val []byte) uint64 {
	return binary.BigEndian.Uint64(val)
}

func ConcatCounts(txCount, eventCount, stateDiffLen uint64, l1Mode L1DAMode) felt.Felt {
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
	return *new(felt.Felt).SetBytes(concatBytes)
}

func gasPricesHash(gasPrices, dataGasPrices, l2GasPrices GasPrice) felt.Felt {
	return crypto.PoseidonArray(
		starknetGasPrices0,
		// gas prices
		gasPrices.PriceInWei,
		gasPrices.PriceInFri,
		// data gas prices
		dataGasPrices.PriceInWei,
		dataGasPrices.PriceInFri,
		// l2 gas prices
		l2GasPrices.PriceInWei,
		l2GasPrices.PriceInFri,
	)
}
