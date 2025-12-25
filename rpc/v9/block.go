package rpcv9

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
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
	Hash             *felt.Felt           `json:"block_hash,omitempty"`
	ParentHash       *felt.Felt           `json:"parent_hash,omitempty"`
	Number           *uint64              `json:"block_number,omitempty"`
	NewRoot          *felt.Felt           `json:"new_root,omitempty"`
	Timestamp        uint64               `json:"timestamp"`
	SequencerAddress *felt.Felt           `json:"sequencer_address,omitempty"`
	L1GasPrice       *rpcv6.ResourcePrice `json:"l1_gas_price"`
	L1DataGasPrice   *rpcv6.ResourcePrice `json:"l1_data_gas_price,omitempty"`
	L1DAMode         *rpcv6.L1DAMode      `json:"l1_da_mode,omitempty"`
	StarknetVersion  string               `json:"starknet_version"`
	L2GasPrice       *rpcv6.ResourcePrice `json:"l2_gas_price"`
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

// BlockTransactionCount returns the number of transactions in a block
// identified by the given BlockID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/9377851884da5c81f757b6ae0ed47e84f9e7c058/api/starknet_api_openrpc.json#L548
func (h *Handler) BlockTransactionCount(id *BlockID) (uint64, *jsonrpc.Error) {
	header, rpcErr := h.blockHeaderByID(id)
	if rpcErr != nil {
		return 0, rpcErr
	}
	return header.TransactionCount, nil
}

// BlockWithTxHashes returns the block information with transaction hashes given a block ID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/9377851884da5c81f757b6ae0ed47e84f9e7c058/api/starknet_api_openrpc.json#L25
func (h *Handler) BlockWithTxHashes(id *BlockID) (*BlockWithTxHashes, *jsonrpc.Error) {
	block, rpcErr := h.blockByID(id)
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
		BlockHeader: AdaptBlockHeader(block.Header),
		TxnHashes:   txnHashes,
	}, nil
}

// BlockWithTxHashes returns the block information with transaction receipts given a block ID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/9377851884da5c81f757b6ae0ed47e84f9e7c058/api/starknet_api_openrpc.json#L99
func (h *Handler) BlockWithReceipts(id *BlockID) (*BlockWithReceipts, *jsonrpc.Error) {
	block, rpcErr := h.blockByID(id)
	if rpcErr != nil {
		return nil, rpcErr
	}

	blockStatus, rpcErr := h.blockStatus(id, block)
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
		// legacy pending block
		if block.ParentHash != nil {
			finalityStatus = TxnAcceptedOnL2
		}
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
	block, rpcErr := h.blockByID(blockID)
	if rpcErr != nil {
		return nil, rpcErr
	}

	txs := make([]*Transaction, len(block.Transactions))
	for index, txn := range block.Transactions {
		txs[index] = AdaptTransaction(txn)
	}

	status, rpcErr := h.blockStatus(blockID, block)
	if rpcErr != nil {
		return nil, rpcErr
	}

	return &BlockWithTxs{
		Status:       status,
		BlockHeader:  AdaptBlockHeader(block.Header),
		Transactions: txs,
	}, nil
}

func (h *Handler) blockStatus(id *BlockID, block *core.Block) (BlockStatus, *jsonrpc.Error) {
	l1H, jsonErr := h.l1Head()
	if jsonErr != nil {
		return 0, jsonErr
	}

	status := BlockAcceptedL2
	if id.IsPreConfirmed() {
		status = BlockPreConfirmed
	} else if isL1Verified(block.Number, l1H) {
		status = BlockAcceptedL1
	}

	return status, nil
}

func AdaptBlockHeader(header *core.Header) BlockHeader {
	blockNumber := &header.Number

	sequencerAddress := header.SequencerAddress
	if sequencerAddress == nil {
		sequencerAddress = &felt.Zero
	}

	var l1DAMode rpcv6.L1DAMode
	switch header.L1DAMode {
	case core.Blob:
		l1DAMode = rpcv6.Blob
	case core.Calldata:
		l1DAMode = rpcv6.Calldata
	}

	var l1DataGasPrice rpcv6.ResourcePrice
	if header.L1DataGasPrice != nil {
		l1DataGasPrice = rpcv6.ResourcePrice{
			InWei: nilToZero(header.L1DataGasPrice.PriceInWei),
			InFri: nilToZero(header.L1DataGasPrice.PriceInFri),
		}
	} else {
		l1DataGasPrice = rpcv6.ResourcePrice{
			InWei: &felt.Zero,
			InFri: &felt.Zero,
		}
	}

	var l2GasPrice rpcv6.ResourcePrice
	if header.L2GasPrice != nil {
		l2GasPrice = rpcv6.ResourcePrice{
			InWei: nilToZero(header.L2GasPrice.PriceInWei),
			InFri: nilToZero(header.L2GasPrice.PriceInFri),
		}
	} else {
		l2GasPrice = rpcv6.ResourcePrice{
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
		L1GasPrice: &rpcv6.ResourcePrice{
			InWei: header.L1GasPriceETH,
			InFri: nilToZero(header.L1GasPriceSTRK),
		},
		L1DataGasPrice:  &l1DataGasPrice,
		L1DAMode:        &l1DAMode,
		StarknetVersion: header.ProtocolVersion,
		L2GasPrice:      &l2GasPrice,
	}
}

func nilToZero(f *felt.Felt) *felt.Felt {
	if f == nil {
		return &felt.Zero
	}
	return f
}
