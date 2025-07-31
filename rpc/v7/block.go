package rpcv7

import (
	"encoding/json"
	"errors"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
)

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L814
type BlockID struct {
	Pending bool
	Latest  bool
	Hash    *felt.Felt
	Number  uint64
}

func (b *BlockID) IsLatest() bool {
	return b.Latest
}

func (b *BlockID) IsPending() bool {
	return b.Pending
}

func (b *BlockID) GetHash() *felt.Felt {
	return b.Hash
}

func (b *BlockID) GetNumber() uint64 {
	return b.Number
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

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1131
type BlockWithTxs struct {
	Status rpcv6.BlockStatus `json:"status,omitempty"`
	rpcv6.BlockHeader
	Transactions []*Transaction `json:"transactions"`
}

type TransactionWithReceipt struct {
	Transaction *Transaction        `json:"transaction"`
	Receipt     *TransactionReceipt `json:"receipt"`
}

type BlockWithReceipts struct {
	Status rpcv6.BlockStatus `json:"status,omitempty"`
	rpcv6.BlockHeader
	Transactions []TransactionWithReceipt `json:"transactions"`
}

/****************************************************
		Block Handlers
*****************************************************/

// BlockWithTxHashes returns the block information with transaction hashes given a block ID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L11
func (h *Handler) BlockWithTxHashes(id BlockID) (*rpcv6.BlockWithTxHashes, *jsonrpc.Error) {
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

	return &rpcv6.BlockWithTxHashes{
		Status:      status,
		BlockHeader: adaptBlockHeader(block.Header),
		TxnHashes:   txnHashes,
	}, nil
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
	if blockStatus == rpcv6.BlockAcceptedL1 {
		finalityStatus = TxnAcceptedOnL1
	}

	txsWithReceipts := make([]TransactionWithReceipt, len(block.Transactions))
	for index, txn := range block.Transactions {
		r := block.Receipts[index]

		txsWithReceipts[index] = TransactionWithReceipt{
			Transaction: AdaptTransaction(txn),
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

func (h *Handler) blockStatus(id BlockID, block *core.Block) (rpcv6.BlockStatus, *jsonrpc.Error) {
	l1H, jsonErr := h.l1Head()
	if jsonErr != nil {
		return 0, jsonErr
	}

	status := rpcv6.BlockAcceptedL2
	if id.Pending {
		status = rpcv6.BlockPending
	} else if isL1Verified(block.Number, l1H) {
		status = rpcv6.BlockAcceptedL1
	}

	return status, nil
}

func adaptBlockHeader(header *core.Header) rpcv6.BlockHeader {
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

	return rpcv6.BlockHeader{
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
	}
}

func nilToZero(f *felt.Felt) *felt.Felt {
	if f == nil {
		return &felt.Zero
	}
	return f
}
