package rpc

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
)

// https://github.com/starkware-libs/starknet-specs/blob/fbf8710c2d2dcdb70a95776f257d080392ad0816/api/starknet_api_openrpc.json#L2353-L2363
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

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L520-L534
type BlockHashAndNumber struct {
	Hash   *felt.Felt `json:"block_hash"`
	Number uint64     `json:"block_number"`
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L814
type BlockID struct {
	Pending bool
	Latest  bool
	Hash    *felt.Felt
	Number  uint64
}

func (b *BlockID) UnmarshalJSON(data []byte) error {
	if string(data) == `"latest"` {
		b.Latest = true
	} else if string(data) == `"pending"` {
		b.Pending = true
	} else {
		jsonObject := make(map[string]json.RawMessage)
		if err := json.Unmarshal(data, &jsonObject); err != nil {
			return err
		}
		hash, ok := jsonObject["block_hash"]
		if ok {
			b.Hash = new(felt.Felt)
			return json.Unmarshal(hash, b.Hash)
		}

		number, ok := jsonObject["block_number"]
		if ok {
			return json.Unmarshal(number, &b.Number)
		}

		return errors.New("cannot unmarshal block id")
	}
	return nil
}

type ResourcePrice struct {
	InFri *felt.Felt `json:"price_in_fri"`
	InWei *felt.Felt `json:"price_in_wei"`
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1072
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
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1131
type BlockWithTxs struct {
	Status BlockStatus `json:"status,omitempty"`
	BlockHeader
	Transactions []*Transaction `json:"transactions"`
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1109
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
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L500
func (h *Handler) BlockNumber() (uint64, *jsonrpc.Error) {
	num, err := h.bcReader.Height()
	if err != nil {
		return 0, ErrNoBlock
	}

	return num, nil
}

// BlockHashAndNumber returns the block hash and number of the latest synced block.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L517
func (h *Handler) BlockHashAndNumber() (*BlockHashAndNumber, *jsonrpc.Error) {
	block, err := h.bcReader.Head()
	if err != nil {
		return nil, ErrNoBlock
	}
	return &BlockHashAndNumber{Number: block.Number, Hash: block.Hash}, nil
}

// BlockWithTxHashes returns the block information with transaction hashes given a block ID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L11
func (h *Handler) BlockWithTxHashes(id BlockID) (*BlockWithTxHashes, *jsonrpc.Error) {
	block, rpcErr := h.blockByID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}

	txnHashes := make([]*felt.Felt, len(block.Transactions))
	for index, txn := range block.Transactions {
		txnHashes[index] = txn.Hash()
	}

	status, rpcErr := h.blockStatus(id, block)
	if rpcErr != nil {
		return nil, rpcErr
	}

	return &BlockWithTxHashes{
		Status:      status,
		BlockHeader: adaptBlockHeader(block.Header),
		TxnHashes:   txnHashes,
	}, nil
}

func (h *Handler) BlockWithTxHashesV0_6(id BlockID) (*BlockWithTxHashes, *jsonrpc.Error) {
	resp, err := h.BlockWithTxHashes(id)
	if err != nil {
		return nil, err
	}

	resp.L1DAMode = nil
	resp.L1DataGasPrice = nil
	return resp, nil
}

// BlockTransactionCount returns the number of transactions in a block
// identified by the given BlockID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L373
func (h *Handler) BlockTransactionCount(id BlockID) (uint64, *jsonrpc.Error) {
	header, rpcErr := h.blockHeaderByID(&id)
	if rpcErr != nil {
		return 0, rpcErr
	}
	return header.TransactionCount, nil
}

func (h *Handler) BlockWithReceipts(id BlockID) (*BlockWithReceipts, *jsonrpc.Error) {
	block, rpcErr := h.blockByID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}

	blockStatus, rpcErr := h.blockStatus(id, block)
	if rpcErr != nil {
		return nil, rpcErr
	}

	finalityStatus := TxnAcceptedOnL2
	if blockStatus == BlockAcceptedL1 {
		finalityStatus = TxnAcceptedOnL1
	}

	txsWithReceipts := make([]TransactionWithReceipt, len(block.Transactions))
	for index, txn := range block.Transactions {
		r := block.Receipts[index]

		txsWithReceipts[index] = TransactionWithReceipt{
			Transaction: AdaptTransaction(txn),
			// block_hash, block_number are optional in BlockWithReceipts response
			Receipt: AdaptReceipt(r, txn, finalityStatus, nil, 0, false),
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
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L44
func (h *Handler) BlockWithTxs(id BlockID) (*BlockWithTxs, *jsonrpc.Error) {
	block, rpcErr := h.blockByID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}

	txs := make([]*Transaction, len(block.Transactions))
	for index, txn := range block.Transactions {
		txs[index] = AdaptTransaction(txn)
	}

	status, rpcErr := h.blockStatus(id, block)
	if rpcErr != nil {
		return nil, rpcErr
	}

	return &BlockWithTxs{
		Status:       status,
		BlockHeader:  adaptBlockHeader(block.Header),
		Transactions: txs,
	}, nil
}

func (h *Handler) BlockWithTxsV0_6(id BlockID) (*BlockWithTxs, *jsonrpc.Error) {
	resp, err := h.BlockWithTxs(id)
	if err != nil {
		return nil, err
	}

	resp.L1DAMode = nil
	resp.L1DataGasPrice = nil
	return resp, nil
}

func (h *Handler) blockStatus(id BlockID, block *core.Block) (BlockStatus, *jsonrpc.Error) {
	l1H, jsonErr := h.l1Head()
	if jsonErr != nil {
		return 0, jsonErr
	}

	status := BlockAcceptedL2
	if id.Pending {
		status = BlockPending
	} else if isL1Verified(block.Number, l1H) {
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

	return BlockHeader{
		Hash:             header.Hash,
		ParentHash:       header.ParentHash,
		Number:           blockNumber,
		NewRoot:          header.GlobalStateRoot,
		Timestamp:        header.Timestamp,
		SequencerAddress: sequencerAddress,
		L1GasPrice: &ResourcePrice{
			InWei: header.GasPrice,
			InFri: nilToZero(header.GasPriceSTRK), // Old block headers will be nil.
		},
		L1DataGasPrice:  &l1DataGasPrice,
		L1DAMode:        &l1DAMode,
		StarknetVersion: header.ProtocolVersion,
	}
}
