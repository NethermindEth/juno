package rpcv9

import (
	"encoding/json"
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

// https://github.com/starkware-libs/starknet-specs/blob/fbf8710c2d2dcdb70a95776f257d080392ad0816/api/starknet_api_openrpc.json#L2353-L2363
type BlockStatus uint8

const (
	BlockPreConfirmed BlockStatus = iota
	BlockAcceptedL2
	BlockAcceptedL1
	BlockRejected
)

func (s BlockStatus) MarshalText() ([]byte, error) {
	switch s {
	case BlockPreConfirmed:
		return []byte("PRE_CONFIRMED"), nil
	case BlockAcceptedL2:
		return []byte("ACCEPTED_ON_L2"), nil
	case BlockAcceptedL1:
		return []byte("ACCEPTED_ON_L1"), nil
	case BlockRejected:
		return []byte("REJECTED"), nil
	default:
		return nil, fmt.Errorf("unknown block status %v", s)
	}
}

type blockIDType uint8

const (
	preConfirmed blockIDType = iota + 1
	latest
	hash
	number
	l1Accepted
)

func (b *blockIDType) String() string {
	switch *b {
	case preConfirmed:
		return "pre_confirmed"
	case latest:
		return "latest"
	case l1Accepted:
		return "l1_accepted"
	case hash:
		return "hash"
	case number:
		return "number"
	default:
		panic(fmt.Sprintf("Unknown blockIdType: %d", b))
	}
}

// https://github.com/starkware-libs/starknet-specs/blob/release/v0.9.0/api/starknet_api_openrpc.json#L741-L759
type BlockHashAndNumber struct {
	Hash   *felt.Felt `json:"block_hash"`
	Number uint64     `json:"block_number"`
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L814
type BlockID struct {
	typeID blockIDType
	data   felt.Felt
}

func BlockIDFromNumber(num uint64) BlockID {
	return BlockID{
		typeID: number,
		data:   felt.Felt([4]uint64{num, 0, 0, 0}),
	}
}

func BlockIDFromHash(blockHash *felt.Felt) BlockID {
	return BlockID{
		typeID: hash,
		data:   *blockHash,
	}
}

func BlockIDPreConfirmed() BlockID {
	return BlockID{
		typeID: preConfirmed,
	}
}

func BlockIDLatest() BlockID {
	return BlockID{
		typeID: latest,
	}
}

func BlockIDL1Accepted() BlockID {
	return BlockID{
		typeID: l1Accepted,
	}
}

func (b *BlockID) Type() blockIDType {
	return b.typeID
}

func (b *BlockID) IsPreConfirmed() bool {
	return b.typeID == preConfirmed
}

func (b *BlockID) IsLatest() bool {
	return b.typeID == latest
}

func (b *BlockID) IsHash() bool {
	return b.typeID == hash
}

func (b *BlockID) IsNumber() bool {
	return b.typeID == number
}

func (b *BlockID) IsL1Accepted() bool {
	return b.typeID == l1Accepted
}

func (b *BlockID) Hash() *felt.Felt {
	if b.typeID != hash {
		panic(fmt.Sprintf("Trying to get hash from block id with type %s", b.typeID.String()))
	}
	return &b.data
}

func (b *BlockID) Number() uint64 {
	if b.typeID != number {
		panic(fmt.Sprintf("Trying to get number from block id with type %s", b.typeID.String()))
	}
	return b.data[0]
}

func (b *BlockID) UnmarshalJSON(data []byte) error {
	var blockTag string
	if err := json.Unmarshal(data, &blockTag); err == nil {
		switch blockTag {
		case "latest":
			b.typeID = latest
		case "pre_confirmed":
			b.typeID = preConfirmed
		case "l1_accepted":
			b.typeID = l1Accepted
		default:
			return fmt.Errorf("unknown block tag '%s'", blockTag)
		}
	} else {
		jsonObject := make(map[string]json.RawMessage)
		if err := json.Unmarshal(data, &jsonObject); err != nil {
			return err
		}
		blockHash, ok := jsonObject["block_hash"]
		if ok {
			b.typeID = hash
			return json.Unmarshal(blockHash, &b.data)
		}

		blockNumber, ok := jsonObject["block_number"]
		if ok {
			b.typeID = number
			return json.Unmarshal(blockNumber, &b.data[0])
		}

		return errors.New("cannot unmarshal block id")
	}
	return nil
}

// Allows omitting ParentHash, pre_confirmed block does not have parentHash
// BLOCK_HEADER
// https://github.com/starkware-libs/starknet-specs/blob/0bf403bfafbfbe0eaa52103a9c7df545bec8f73b/api/starknet_api_openrpc.json#L1622
// PRE_CONFIRMED_BLOCK_HEADER
// https://github.com/starkware-libs/starknet-specs/blob/0bf403bfafbfbe0eaa52103a9c7df545bec8f73b/api/starknet_api_openrpc.json#L1636
type BlockHeader struct {
	Hash             *felt.Felt    `json:"block_hash,omitempty"`
	ParentHash       *felt.Felt    `json:"parent_hash,omitempty"`
	Number           *uint64       `json:"block_number,omitempty"`
	NewRoot          *felt.Felt    `json:"new_root,omitempty"`
	Timestamp        uint64        `json:"timestamp"`
	SequencerAddress *felt.Felt    `json:"sequencer_address,omitempty"`
	L1GasPrice       ResourcePrice `json:"l1_gas_price"`
	L1DataGasPrice   ResourcePrice `json:"l1_data_gas_price"`
	L1DAMode         L1DAMode      `json:"l1_da_mode"`
	StarknetVersion  string        `json:"starknet_version"`
	L2GasPrice       ResourcePrice `json:"l2_gas_price"`
}

type ResourcePrice struct {
	InFri *felt.Felt `json:"price_in_fri"`
	InWei *felt.Felt `json:"price_in_wei"`
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

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1131
type BlockWithTxs struct {
	Status BlockStatus `json:"status,omitempty"`
	BlockHeader
	Transactions []*Transaction `json:"transactions"`
}

// https://github.com/starkware-libs/starknet-specs/blob/9377851884da5c81f757b6ae0ed47e84f9e7c058/api/starknet_api_openrpc.json#L43
type BlockWithTxHashes struct {
	Status BlockStatus `json:"status,omitempty"`
	BlockHeader
	TxnHashes []felt.Felt `json:"transactions"`
}

type TransactionWithReceipt struct {
	Transaction *Transaction        `json:"transaction"`
	Receipt     *TransactionReceipt `json:"receipt"`
}

type BlockWithReceipts struct {
	Status BlockStatus `json:"status,omitempty"`
	BlockHeader
	Transactions []TransactionWithReceipt `json:"transactions"`
}

/****************************************************
		Block Handlers
*****************************************************/

// BlockNumber returns the latest synced block number.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/release/v0.9.0/api/starknet_api_openrpc.json#L720
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
// https://github.com/starkware-libs/starknet-specs/blob/release/v0.9.0/api/starknet_api_openrpc.json#L738
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
// https://github.com/starkware-libs/starknet-specs/blob/9377851884da5c81f757b6ae0ed47e84f9e7c058/api/starknet_api_openrpc.json#L548
func (h *Handler) BlockTransactionCount(id *BlockID) (uint64, *jsonrpc.Error) {
	var count uint64
	var err error
	switch id.Type() {
	case preConfirmed:
		var chain preconfirmed.ChainReader
		chain, err = h.syncReader.PreConfirmedChain()
		if err == nil {
			count = chain.Head().Block.Header.TransactionCount
		}
	case latest:
		var height uint64
		height, err = h.bcReader.Height()
		if err == nil {
			count, err = h.bcReader.BlockTransactionCountByNumber(height)
		}
	case hash:
		var blockNumber uint64
		blockNumber, err = h.bcReader.BlockNumberByHash(id.Hash())
		if err == nil {
			count, err = h.bcReader.BlockTransactionCountByNumber(blockNumber)
		}
	case number:
		count, err = h.bcReader.BlockTransactionCountByNumber(id.Number())
	case l1Accepted:
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
// https://github.com/starkware-libs/starknet-specs/blob/9377851884da5c81f757b6ae0ed47e84f9e7c058/api/starknet_api_openrpc.json#L25
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
			BlockHeader: AdaptBlockHeader(preConfirmed.Block.Header),
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

	return &BlockWithTxHashes{
		Status:      status,
		BlockHeader: AdaptBlockHeader(header),
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
// https://github.com/starkware-libs/starknet-specs/blob/9377851884da5c81f757b6ae0ed47e84f9e7c058/api/starknet_api_openrpc.json#L99
func (h *Handler) BlockWithReceipts(id *BlockID) (*BlockWithReceipts, *jsonrpc.Error) {
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

		t := AdaptTransaction(txn)
		t.Hash = nil
		txsWithReceipts[index] = TransactionWithReceipt{
			Transaction: t,
			Receipt:     AdaptReceipt(r, txn, finalityStatus),
		}
	}

	return &BlockWithReceipts{
		Status:       blockStatus,
		BlockHeader:  AdaptBlockHeader(block.Header),
		Transactions: txsWithReceipts,
	}, nil
}

// BlockWithTxs returns the block information with full transactions given a block ID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/9377851884da5c81f757b6ae0ed47e84f9e7c058/api/starknet_api_openrpc.json#L62
func (h *Handler) BlockWithTxs(blockID *BlockID) (*BlockWithTxs, *jsonrpc.Error) {
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
			BlockHeader:  AdaptBlockHeader(preConfirmed.Block.Header),
			Transactions: adaptTransactions(preConfirmed.Block.Transactions),
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

	return &BlockWithTxs{
		Status:       status,
		BlockHeader:  AdaptBlockHeader(header),
		Transactions: adaptTransactions(blockTransactions),
	}, nil
}

// adaptTransactions sizes the result from the transactions themselves rather than the header's
// count, so a header and a transaction list that disagree cannot index out of range.
func adaptTransactions(transactions []core.Transaction) []*Transaction {
	adapted := make([]*Transaction, len(transactions))
	for index, transaction := range transactions {
		adapted[index] = AdaptTransaction(transaction)
	}
	return adapted
}

func (h *Handler) blockStatus(id *BlockID, blockNumber uint64) (BlockStatus, *jsonrpc.Error) {
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

func AdaptBlockHeader(header *core.Header) BlockHeader {
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
			InWei: nilToZero(header.L1DataGasPrice.PriceInWei),
			InFri: nilToZero(header.L1DataGasPrice.PriceInFri),
		}
	} else {
		l1DataGasPrice = ResourcePrice{
			InWei: &felt.Zero,
			InFri: &felt.Zero,
		}
	}

	var l2GasPrice ResourcePrice
	if header.L2GasPrice != nil {
		l2GasPrice = ResourcePrice{
			InWei: nilToZero(header.L2GasPrice.PriceInWei),
			InFri: nilToZero(header.L2GasPrice.PriceInFri),
		}
	} else {
		l2GasPrice = ResourcePrice{
			InWei: &felt.Zero,
			InFri: &felt.Zero,
		}
	}

	return BlockHeader{
		Hash:             header.Hash,
		ParentHash:       header.ParentHash,
		Number:           &header.Number,
		NewRoot:          header.GlobalStateRoot,
		Timestamp:        header.Timestamp,
		SequencerAddress: sequencerAddress,
		L1GasPrice: ResourcePrice{
			InWei: nilToZero(header.L1GasPriceETH),
			InFri: nilToZero(header.L1GasPriceSTRK),
		},
		L1DataGasPrice:  l1DataGasPrice,
		L1DAMode:        l1DAMode,
		StarknetVersion: header.ProtocolVersion,
		L2GasPrice:      l2GasPrice,
	}
}

func nilToZero(f *felt.Felt) *felt.Felt {
	if f == nil {
		return &felt.Zero
	}
	return f
}
