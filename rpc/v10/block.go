package rpcv10

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/pending"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/sync/preconfirmed"
)

type ResourcePrice struct {
	InWei *felt.Felt `json:"price_in_wei"`
	InFri *felt.Felt `json:"price_in_fri,omitempty"`
}

type L1DAMode uint8

const (
	Blob L1DAMode = iota
	Calldata
)

func (l L1DAMode) MarshalText() ([]byte, error) {
	switch l {
	case Blob:
		return []byte("BLOB"), nil
	case Calldata:
		return []byte("CALLDATA"), nil
	default:
		return nil, fmt.Errorf("unknown L1DAMode value = %v", l)
	}
}

// BLOCK_HEADER
// https://github.com/starkware-libs/starknet-specs/blob/cce1563eff702c87590bad3a48382d2febf1f7d9/api/starknet_api_openrpc.json#L1591
// PRE_CONFIRMED_BLOCK_HEADER
// https://github.com/starkware-libs/starknet-specs/blob/cce1563eff702c87590bad3a48382d2febf1f7d9/api/starknet_api_openrpc.json#L1711
type BlockHeader struct {
	Hash                  *felt.Felt    `json:"block_hash,omitempty"`
	ParentHash            *felt.Felt    `json:"parent_hash,omitempty"`
	Number                *uint64       `json:"block_number,omitempty"`
	NewRoot               *felt.Felt    `json:"new_root,omitempty"`
	Timestamp             uint64        `json:"timestamp"`
	SequencerAddress      *felt.Felt    `json:"sequencer_address,omitempty"`
	L1GasPrice            ResourcePrice `json:"l1_gas_price"`
	L1DataGasPrice        ResourcePrice `json:"l1_data_gas_price"`
	L1DAMode              L1DAMode      `json:"l1_da_mode"`
	StarknetVersion       string        `json:"starknet_version"`
	L2GasPrice            ResourcePrice `json:"l2_gas_price"`
	TransactionCommitment *felt.Hash    `json:"transaction_commitment,omitempty"`
	EventCommitment       *felt.Hash    `json:"event_commitment,omitempty"`
	ReceiptCommitment     *felt.Hash    `json:"receipt_commitment,omitempty"`
	StateDiffCommitment   *felt.Hash    `json:"state_diff_commitment,omitempty"`
	EventCount            *uint64       `json:"event_count,omitempty"`
	TransactionCount      *uint64       `json:"transaction_count,omitempty"`
	StateDiffLength       *uint64       `json:"state_diff_length,omitempty"`
}

// https://github.com/starkware-libs/starknet-specs/blob/cce1563eff702c87590bad3a48382d2febf1f7d9/api/starknet_api_openrpc.json#L1794
type BlockWithTxs struct {
	Status BlockStatus `json:"status,omitempty"`
	BlockHeader
	Transactions []*Transaction `json:"transactions"`
}

// https://github.com/starkware-libs/starknet-specs/blob/cce1563eff702c87590bad3a48382d2febf1f7d9/api/starknet_api_openrpc.json#L1769
type BlockWithTxHashes struct {
	Status BlockStatus `json:"status,omitempty"`
	BlockHeader
	TxnHashes []felt.Felt `json:"transactions"`
}

// TransactionWithReceipt represents a transaction with its receipt
type TransactionWithReceipt struct {
	Transaction *Transaction        `json:"transaction"`
	Receipt     *TransactionReceipt `json:"receipt"`
}

// https://github.com/starkware-libs/starknet-specs/blob/cce1563eff702c87590bad3a48382d2febf1f7d9/api/starknet_api_openrpc.json#L1819
type BlockWithReceipts struct {
	Status BlockStatus `json:"status,omitempty"`
	BlockHeader
	Transactions []TransactionWithReceipt `json:"transactions"`
}

// https://github.com/starkware-libs/starknet-specs/blob/v0.10.3/api/starknet_api_openrpc.json#L830-L848
type BlockHashAndNumber struct {
	Hash   *felt.Felt `json:"block_hash"`
	Number uint64     `json:"block_number"`
}

/****************************************************
		Block Handlers
*****************************************************/

// BlockNumber returns the latest synced block number.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/v0.10.3/api/starknet_api_openrpc.json#L809
func (h *Handler) BlockNumber() (uint64, *jsonrpc.Error) {
	num, err := h.bcReader.Height()
	if err != nil {
		return 0, rpccore.ErrNoBlock
	}

	return num, nil
}

// BlockHashAndNumber returns the block hash and number of the latest synced block.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/v0.10.3/api/starknet_api_openrpc.json#L827
func (h *Handler) BlockHashAndNumber() (*BlockHashAndNumber, *jsonrpc.Error) {
	block, err := h.bcReader.Head()
	if err != nil {
		return nil, rpccore.ErrNoBlock
	}
	return &BlockHashAndNumber{Number: block.Number, Hash: block.Hash}, nil
}

// BlockTransactionCount returns the number of transactions in a block
// identified by the given BlockID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/v0.10.3/api/starknet_api_openrpc.json#L622
func (h *Handler) BlockTransactionCount(id *BlockID) (uint64, *jsonrpc.Error) {
	var count uint64
	var err error
	switch {
	case id.IsPreConfirmed():
		var reader preconfirmed.ChainReader
		reader, err = h.syncReader.PreConfirmedChain()
		if err == nil {
			count = reader.Head().GetHeader().TransactionCount
		}
	case id.IsLatest():
		var height uint64
		height, err = h.bcReader.Height()
		if err == nil {
			count, err = h.bcReader.BlockTransactionCountByNumber(height)
		}
	case id.IsHash():
		var number uint64
		number, err = h.bcReader.BlockNumberByHash(id.Hash())
		if err == nil {
			count, err = h.bcReader.BlockTransactionCountByNumber(number)
		}
	case id.IsNumber():
		count, err = h.bcReader.BlockTransactionCountByNumber(id.Number())
	case id.IsL1Accepted():
		var blockNumber uint64
		blockNumber, err = h.l1AcceptedBlockNumber()
		if err == nil {
			count, err = h.bcReader.BlockTransactionCountByNumber(blockNumber)
		}
	default:
		panic("unknown block type id")
	}

	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) || errors.Is(err, pending.ErrPreConfirmedNotFound) {
			return 0, rpccore.ErrBlockNotFound
		}
		return 0, rpccore.ErrInternal.CloneWithData(err)
	}
	return count, nil
}

// BlockWithTxHashes returns the block information with transaction hashes given a block ID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/cce1563eff702c87590bad3a48382d2febf1f7d9/api/starknet_api_openrpc.json#L25
func (h *Handler) BlockWithTxHashes(id *BlockID) (*BlockWithTxHashes, *jsonrpc.Error) {
	if id.IsPreConfirmed() {
		preConfirmedChain, err := h.syncReader.PreConfirmedChain()
		if err != nil {
			if errors.Is(err, db.ErrKeyNotFound) || errors.Is(err, pending.ErrPreConfirmedNotFound) {
				return nil, rpccore.ErrBlockNotFound
			}
			return nil, rpccore.ErrInternal.CloneWithData(err)
		}
		preConfirmed := preConfirmedChain.Head()
		if preConfirmed == nil {
			return nil, rpccore.ErrBlockNotFound
		}
		return &BlockWithTxHashes{
			Status:      BlockPreConfirmed,
			BlockHeader: AdaptBlockHeader(preConfirmed.Block.Header, nil, nil),
			TxnHashes:   transactionHashesOf(preConfirmed.Block.Transactions),
		}, nil
	}

	header, rpcErr := h.blockHeaderByID(id)
	if rpcErr != nil {
		return nil, rpcErr
	}

	transactionHashes, err := h.bcReader.TransactionHashesByBlockNumber(header.Number)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, rpccore.ErrBlockNotFound
		}
		return nil, rpccore.ErrInternal.CloneWithData(err)
	}

	status, rpcErr := h.blockStatus(id, header.Number)
	if rpcErr != nil {
		return nil, rpcErr
	}

	commitments, err := h.bcReader.BlockCommitmentsByNumber(header.Number)
	if err != nil {
		return nil, rpccore.ErrInternal.CloneWithData(err)
	}

	return &BlockWithTxHashes{
		Status:      status,
		BlockHeader: AdaptBlockHeader(header, commitments, stateDiff),
		TxnHashes:   transactionHashes,
	}, nil
}

// transactionHashesOf collects each transaction's hash, for blocks served from memory where the
// transactions are already decoded.
func transactionHashesOf(transactions []core.Transaction) []felt.Felt {
	hashes := make([]felt.Felt, len(transactions))
	for index, transaction := range transactions {
		hashes[index] = *transaction.Hash()
	}
	return hashes
}

// BlockWithReceipts returns the block information with transaction receipts given a block ID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/cce1563eff702c87590bad3a48382d2febf1f7d9/api/starknet_api_openrpc.json#L99
func (h *Handler) BlockWithReceipts(
	id *BlockID,
	responseFlags ResponseFlags,
) (*BlockWithReceipts, *jsonrpc.Error) {
	includeProofFacts := responseFlags.IncludeProofFacts

	block, rpcErr := h.blockByID(id)
	if rpcErr != nil {
		return nil, rpcErr
	}

	blockStatus, rpcErr := h.blockStatus(id, block.Number)
	if rpcErr != nil {
		return nil, rpcErr
	}

	var finalityStatus TxnFinalityStatus
	switch s := blockStatus; s {
	case BlockAcceptedL1:
		finalityStatus = TxnAcceptedOnL1
	case BlockAcceptedL2:
		finalityStatus = TxnAcceptedOnL2
	case BlockPreConfirmed:
		finalityStatus = TxnPreConfirmed
	default:
		return nil, rpccore.ErrInternal.CloneWithData(fmt.Errorf("unknown block status '%v'", s))
	}

	txsWithReceipts := make([]TransactionWithReceipt, len(block.Transactions))
	for index, txn := range block.Transactions {
		r := block.Receipts[index]

		adaptedTx := AdaptTransaction(txn, includeProofFacts)
		adaptedTx.Hash = nil
		txsWithReceipts[index] = TransactionWithReceipt{
			Transaction: &adaptedTx,
			// block_hash, block_number are optional in BlockWithReceipts response
			Receipt: AdaptReceipt(r, txn, finalityStatus),
		}
	}

	var commitments *core.BlockCommitments
	var err error
	if block.Hash != nil {
		commitments, err = h.bcReader.BlockCommitmentsByNumber(block.Number)
		if err != nil {
			return nil, rpccore.ErrInternal.CloneWithData(err)
		}
	}

	return &BlockWithReceipts{
		Status:       blockStatus,
		BlockHeader:  AdaptBlockHeader(block.Header, commitments),
		Transactions: txsWithReceipts,
	}, nil
}

// BlockWithTxs returns the block information with full transactions given a block ID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/cce1563eff702c87590bad3a48382d2febf1f7d9/api/starknet_api_openrpc.json#L62
func (h *Handler) BlockWithTxs(
	blockID *BlockID,
	responseFlags ResponseFlags,
) (*BlockWithTxs, *jsonrpc.Error) {
	includeProofFacts := responseFlags.IncludeProofFacts

	if blockID.IsPreConfirmed() {
		preConfirmedChain, err := h.syncReader.PreConfirmedChain()
		if err != nil {
			if errors.Is(err, db.ErrKeyNotFound) || errors.Is(err, pending.ErrPreConfirmedNotFound) {
				return nil, rpccore.ErrBlockNotFound
			}
			return nil, rpccore.ErrInternal.CloneWithData(err)
		}
		preConfirmed := preConfirmedChain.Head()
		if preConfirmed == nil {
			return nil, rpccore.ErrBlockNotFound
		}
		return &BlockWithTxs{
			Status:       BlockPreConfirmed,
			BlockHeader:  AdaptBlockHeader(preConfirmed.Block.Header, nil, nil),
			Transactions: adaptTransactions(preConfirmed.Block.Transactions, includeProofFacts),
		}, nil
	}

	header, rpcErr := h.blockHeaderByID(blockID)
	if rpcErr != nil {
		return nil, rpcErr
	}

	blockTransactions, err := h.bcReader.TransactionsByBlockNumber(header.Number)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, rpccore.ErrBlockNotFound
		}
		return nil, rpccore.ErrInternal.CloneWithData(err)
	}

	status, rpcErr := h.blockStatus(blockID, header.Number)
	if rpcErr != nil {
		return nil, rpcErr
	}

	commitments, err := h.bcReader.BlockCommitmentsByNumber(header.Number)
	if err != nil {
		return nil, rpccore.ErrInternal.CloneWithData(err)
	}

	return &BlockWithTxs{
		Status:       status,
		BlockHeader:  AdaptBlockHeader(header, commitments, stateDiff),
		Transactions: adaptTransactions(blockTransactions, includeProofFacts),
	}, nil
}

func adaptTransactions(transactions []core.Transaction, includeProofFacts bool) []*Transaction {
	adapted := make([]*Transaction, len(transactions))
	for index, transaction := range transactions {
		adaptedTransaction := AdaptTransaction(transaction, includeProofFacts)
		adapted[index] = &adaptedTransaction
	}
	return adapted
}

func (h *Handler) blockStatus(
	id *BlockID,
	blockNumber uint64,
) (BlockStatus, *jsonrpc.Error) {
	if id.IsPreConfirmed() {
		return BlockPreConfirmed, nil
	}

	l1H, jsonErr := h.l1Head()
	if jsonErr != nil {
		return 0, jsonErr
	}

	if isL1Verified(blockNumber, l1H) {
		return BlockAcceptedL1, nil
	}

	return BlockAcceptedL2, nil
}

func AdaptBlockHeader(
	header *core.Header,
	commitments *core.BlockCommitments,
) BlockHeader {
	sequencerAddress := header.SequencerAddress
	if sequencerAddress == nil {
		sequencerAddress = &felt.Zero
	}

	var l1DAMode L1DAMode
	switch header.L1DAMode {
	case core.Blob:
		l1DAMode = Blob
	case core.Calldata:
		l1DAMode = Calldata
	}

	var l1DataGasPrice ResourcePrice
	if header.L1DataGasPrice != nil {
		l1DataGasPrice = ResourcePrice{
			InWei: nilToOne(header.L1DataGasPrice.PriceInWei),
			InFri: nilToOne(header.L1DataGasPrice.PriceInFri),
		}
	} else {
		l1DataGasPrice = ResourcePrice{
			InWei: &felt.One,
			InFri: &felt.One,
		}
	}

	var l2GasPrice ResourcePrice
	if header.L2GasPrice != nil {
		l2GasPrice = ResourcePrice{
			InWei: nilToOne(header.L2GasPrice.PriceInWei),
			InFri: nilToOne(header.L2GasPrice.PriceInFri),
		}
	} else {
		l2GasPrice = ResourcePrice{
			InWei: &felt.One,
			InFri: &felt.One,
		}
	}

	blockHeader := BlockHeader{
		Hash:             header.Hash,
		ParentHash:       header.ParentHash,
		Number:           &header.Number,
		NewRoot:          header.GlobalStateRoot,
		Timestamp:        header.Timestamp,
		SequencerAddress: sequencerAddress,
		L1GasPrice: ResourcePrice{
			InWei: nilToOne(header.L1GasPriceETH),
			InFri: nilToOne(header.L1GasPriceSTRK),
		},
		L1DataGasPrice:  l1DataGasPrice,
		L1DAMode:        l1DAMode,
		StarknetVersion: header.ProtocolVersion,
		L2GasPrice:      l2GasPrice,
	}

	// Only populate commitment fields for blocks with commitments
	if header.Hash != nil {
		blockHeader.TransactionCommitment = (*felt.Hash)(nilToZero(commitments.TransactionCommitment))
		blockHeader.EventCommitment = (*felt.Hash)(nilToZero(commitments.EventCommitment))
		blockHeader.ReceiptCommitment = (*felt.Hash)(nilToZero(commitments.ReceiptCommitment))
		blockHeader.StateDiffCommitment = (*felt.Hash)(nilToZero(commitments.StateDiffCommitment))

		// Populate counts from header
		blockHeader.TransactionCount = &header.TransactionCount
		blockHeader.EventCount = &header.EventCount

		blockHeader.StateDiffLength = &commitments.StateDiffLength
	}

	return blockHeader
}

func nilToZero(f *felt.Felt) *felt.Felt {
	if f == nil {
		return &felt.Zero
	}
	return f
}

func nilToOne(f *felt.Felt) *felt.Felt {
	if f == nil {
		return &felt.One
	}
	return f
}
