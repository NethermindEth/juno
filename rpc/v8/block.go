package rpcv8

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
)

type blockIDType uint8

const (
	pending blockIDType = iota + 1
	latest
	hash
	number
)

func (b *blockIDType) String() string {
	switch *b {
	case pending:
		return "pending"
	case latest:
		return "latest"
	case hash:
		return "hash"
	case number:
		return "number"
	default:
		panic(fmt.Sprintf("Unknown blockIdType: %d", b))
	}
}

// https://github.com/starkware-libs/starknet-specs/blob/v0.8.1/api/starknet_api_openrpc.json#L733-L751
type BlockHashAndNumber struct {
	Hash   *felt.Felt `json:"block_hash"`
	Number uint64     `json:"block_number"`
}

// https://github.com/starkware-libs/starknet-specs/blob/v0.8.1/api/starknet_api_openrpc.json#L1190
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

func BlockIDPending() BlockID {
	return BlockID{
		typeID: pending,
	}
}

func (b *BlockID) Type() blockIDType {
	return b.typeID
}

func (b *BlockID) IsPending() bool {
	return b.typeID == pending
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
		case "pending":
			b.typeID = pending
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

// https://github.com/starkware-libs/starknet-specs/blob/v0.8.1/api/starknet_api_openrpc.json#L1544
type BlockHeader struct {
	Hash             *felt.Felt     `json:"block_hash,omitempty"`
	ParentHash       *felt.Felt     `json:"parent_hash"`
	Number           *uint64        `json:"block_number,omitempty"`
	NewRoot          *felt.Felt     `json:"new_root,omitempty"`
	Timestamp        uint64         `json:"timestamp"`
	SequencerAddress *felt.Felt     `json:"sequencer_address,omitempty"`
	L1GasPrice       *ResourcePrice `json:"l1_gas_price"`
	L1DataGasPrice   *ResourcePrice `json:"l1_data_gas_price,omitempty"`
	L1DAMode         *L1DAMode      `json:"l1_da_mode,omitempty"`
	StarknetVersion  string         `json:"starknet_version"`
	L2GasPrice       *ResourcePrice `json:"l2_gas_price"`
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

// https://github.com/starkware-libs/starknet-specs/blob/v0.8.1/api/starknet_api_openrpc.json#L3129
type BlockStatus uint8

const (
	BlockPending BlockStatus = iota
	BlockAcceptedL2
	BlockAcceptedL1
	BlockRejected
)

func (s BlockStatus) MarshalText() ([]byte, error) {
	switch s {
	case BlockPending:
		return []byte("PENDING"), nil
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

// https://github.com/starkware-libs/starknet-specs/blob/v0.8.1/api/starknet_api_openrpc.json#L1705
type BlockWithTxs struct {
	Status BlockStatus `json:"status,omitempty"`
	BlockHeader
	Transactions []*Transaction `json:"transactions"`
}

// https://github.com/starkware-libs/starknet-specs/blob/v0.8.1/api/starknet_api_openrpc.json#L1680
type BlockWithTxHashes struct {
	Status BlockStatus `json:"status,omitempty"`
	BlockHeader
	TxnHashes []*felt.Felt `json:"transactions"`
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
// https://github.com/starkware-libs/starknet-specs/blob/v0.8.1/api/starknet_api_openrpc.json#L712
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
// https://github.com/starkware-libs/starknet-specs/blob/v0.8.1/api/starknet_api_openrpc.json#L730
func (h *Handler) BlockHashAndNumber() (*BlockHashAndNumber, *jsonrpc.Error) {
	block, err := h.bcReader.Head()
	if err != nil {
		return nil, rpccore.ErrNoBlock
	}
	return &BlockHashAndNumber{Number: block.Number, Hash: block.Hash}, nil
}

// BlockWithTxHashes returns the block information with transaction hashes given a block ID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/v0.8.1/api/starknet_api_openrpc.json#L25
func (h *Handler) BlockWithTxHashes(id *BlockID) (*BlockWithTxHashes, *jsonrpc.Error) {
	header, rpcErr := h.blockHeaderByID(id)
	if rpcErr != nil {
		return nil, rpcErr
	}

	var numID BlockID
	if id.IsPending() {
		numID = *id
	} else {
		numID = BlockIDFromNumber(header.Number)
	}
	blockTxns, rpcErr := h.blockTxnsByNumber(&numID)
	if rpcErr != nil {
		return nil, rpcErr
	}

	txnHashes := make([]*felt.Felt, header.TransactionCount)
	for index, txn := range blockTxns {
		txnHashes[index] = txn.Hash()
	}

	status, rpcErr := h.blockStatus(id, header.Number)
	if rpcErr != nil {
		return nil, rpcErr
	}

	return &BlockWithTxHashes{
		Status:      status,
		BlockHeader: adaptBlockHeader(header),
		TxnHashes:   txnHashes,
	}, nil
}

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
	switch blockStatus {
	case BlockAcceptedL1:
		finalityStatus = TxnAcceptedOnL1
	case BlockPending:
		finalityStatus = TxnPending
	default:
		finalityStatus = TxnAcceptedOnL2
	}

	txsWithReceipts := make([]TransactionWithReceipt, len(block.Transactions))
	for index, txn := range block.Transactions {
		r := block.Receipts[index]

		t := AdaptTransaction(txn)
		t.Hash = nil
		txsWithReceipts[index] = TransactionWithReceipt{
			Transaction: t,
			// block_hash, block_number are optional in BlockWithReceipts response
			Receipt: AdaptReceipt(r, txn, finalityStatus, nil, 0),
		}
	}

	return &BlockWithReceipts{
		Status:       blockStatus,
		BlockHeader:  adaptBlockHeader(block.Header),
		Transactions: txsWithReceipts,
	}, nil
}

// BlockWithTxs returns the block information with full transactions given a block ID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/v0.8.1/api/starknet_api_openrpc.json#L62
func (h *Handler) BlockWithTxs(blockID *BlockID) (*BlockWithTxs, *jsonrpc.Error) {
	header, rpcErr := h.blockHeaderByID(blockID)
	if rpcErr != nil {
		return nil, rpcErr
	}

	var numID BlockID
	if blockID.IsPending() {
		numID = *blockID
	} else {
		numID = BlockIDFromNumber(header.Number)
	}
	blockTxns, rpcErr := h.blockTxnsByNumber(&numID)
	if rpcErr != nil {
		return nil, rpcErr
	}

	txs := make([]*Transaction, header.TransactionCount)
	for index, txn := range blockTxns {
		txs[index] = AdaptTransaction(txn)
	}

	status, rpcErr := h.blockStatus(blockID, header.Number)
	if rpcErr != nil {
		return nil, rpcErr
	}

	return &BlockWithTxs{
		Status:       status,
		BlockHeader:  adaptBlockHeader(header),
		Transactions: txs,
	}, nil
}

// BlockTransactionCount returns the number of transactions in a block
// identified by the given BlockID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/v0.8.1/api/starknet_api_openrpc.json#L531
func (h *Handler) BlockTransactionCount(id BlockID) (uint64, *jsonrpc.Error) {
	header, rpcErr := h.blockHeaderByID(&id)
	if rpcErr != nil {
		return 0, rpcErr
	}
	return header.TransactionCount, nil
}

func (h *Handler) blockStatus(id *BlockID, blockNumber uint64) (BlockStatus, *jsonrpc.Error) {
	l1H, jsonErr := h.l1Head()
	if jsonErr != nil {
		return 0, jsonErr
	}

	status := BlockAcceptedL2
	if id.IsPending() {
		status = BlockPending
	} else if isL1Verified(blockNumber, l1H) {
		status = BlockAcceptedL1
	}

	return status, nil
}

func adaptBlockHeader(header *core.Header) BlockHeader {
	var blockNumber *uint64
	// if header.Hash == nil it's a pending block
	if header.Hash != nil {
		blockNumber = &header.Number
	}

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
		Number:           blockNumber,
		NewRoot:          header.GlobalStateRoot,
		Timestamp:        header.Timestamp,
		SequencerAddress: sequencerAddress,
		L1GasPrice: &ResourcePrice{
			InWei: header.L1GasPriceETH,
			InFri: nilToZero(header.L1GasPriceSTRK),
		},
		L1DataGasPrice:  &l1DataGasPrice,
		L2GasPrice:      &l2GasPrice,
		L1DAMode:        &l1DAMode,
		StarknetVersion: header.ProtocolVersion,
	}
}

func nilToZero(f *felt.Felt) *felt.Felt {
	if f == nil {
		return &felt.Zero
	}
	return f
}
