package rpcv8

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
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
	if string(data) == `"latest"` {
		b.typeID = latest
	} else if string(data) == `"pending"` {
		b.typeID = pending
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

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1072
type BlockHeader struct {
	rpcv6.BlockHeader
	L2GasPrice *rpcv6.ResourcePrice `json:"l2_gas_price"`
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1131
type BlockWithTxs struct {
	Status rpcv6.BlockStatus `json:"status,omitempty"`
	BlockHeader
	Transactions []*Transaction `json:"transactions"`
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1109
type BlockWithTxHashes struct {
	Status rpcv6.BlockStatus `json:"status,omitempty"`
	BlockHeader
	TxnHashes []*felt.Felt `json:"transactions"`
}

type TransactionWithReceipt struct {
	Transaction *Transaction        `json:"transaction"`
	Receipt     *TransactionReceipt `json:"receipt"`
}

type BlockWithReceipts struct {
	Status rpcv6.BlockStatus `json:"status,omitempty"`
	BlockHeader
	Transactions []TransactionWithReceipt `json:"transactions"`
}

/****************************************************
		Block Handlers
*****************************************************/

// BlockWithTxHashes returns the block information with transaction hashes given a block ID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L11
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
		BlockHeader: adaptBlockHeader(block.Header),
		TxnHashes:   txnHashes,
	}, nil
}

func (h *Handler) BlockWithReceipts(id *BlockID) (*BlockWithReceipts, *jsonrpc.Error) {
	block, rpcErr := h.blockByID(id)
	if rpcErr != nil {
		return nil, rpcErr
	}

	blockStatus, rpcErr := h.blockStatus(id, block)
	if rpcErr != nil {
		return nil, rpcErr
	}

	finalityStatus := TxnAcceptedOnL2
	if blockStatus == rpcv6.BlockAcceptedL1 {
		finalityStatus = TxnAcceptedOnL1
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
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L44
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
		BlockHeader:  adaptBlockHeader(block.Header),
		Transactions: txs,
	}, nil
}

func (h *Handler) blockStatus(id *BlockID, block *core.Block) (rpcv6.BlockStatus, *jsonrpc.Error) {
	l1H, jsonErr := h.l1Head()
	if jsonErr != nil {
		return 0, jsonErr
	}

	status := rpcv6.BlockAcceptedL2
	if id.IsPending() {
		status = rpcv6.BlockPending
	} else if isL1Verified(block.Number, l1H) {
		status = rpcv6.BlockAcceptedL1
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
		BlockHeader: rpcv6.BlockHeader{
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
		},
		L2GasPrice: &l2GasPrice,
	}
}

func nilToZero(f *felt.Felt) *felt.Felt {
	if f == nil {
		return &felt.Zero
	}
	return f
}
