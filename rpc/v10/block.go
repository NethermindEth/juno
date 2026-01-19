package rpcv10

import (
	"encoding/json"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
)

// BLOCK_HEADER
// https://github.com/starkware-libs/starknet-specs/blob/cce1563eff702c87590bad3a48382d2febf1f7d9/api/starknet_api_openrpc.json#L1591
// PRE_CONFIRMED_BLOCK_HEADER
// https://github.com/starkware-libs/starknet-specs/blob/cce1563eff702c87590bad3a48382d2febf1f7d9/api/starknet_api_openrpc.json#L1711
type BlockHeader struct {
	Hash                  *felt.Felt           `json:"block_hash,omitempty"`
	ParentHash            *felt.Felt           `json:"parent_hash,omitempty"`
	Number                *uint64              `json:"block_number,omitempty"`
	NewRoot               *felt.Felt           `json:"new_root,omitempty"`
	Timestamp             uint64               `json:"timestamp"`
	SequencerAddress      *felt.Felt           `json:"sequencer_address,omitempty"`
	L1GasPrice            *rpcv6.ResourcePrice `json:"l1_gas_price"`
	L1DataGasPrice        *rpcv6.ResourcePrice `json:"l1_data_gas_price,omitempty"`
	L1DAMode              *rpcv6.L1DAMode      `json:"l1_da_mode,omitempty"`
	StarknetVersion       string               `json:"starknet_version"`
	L2GasPrice            *rpcv6.ResourcePrice `json:"l2_gas_price"`
	TransactionCommitment *felt.Hash           `json:"transaction_commitment,omitempty"`
	EventCommitment       *felt.Hash           `json:"event_commitment,omitempty"`
	ReceiptCommitment     *felt.Hash           `json:"receipt_commitment,omitempty"`
	StateDiffCommitment   *felt.Hash           `json:"state_diff_commitment,omitempty"`
	EventCount            *uint64              `json:"event_count,omitempty"`
	TransactionCount      *uint64              `json:"transaction_count,omitempty"`
	StateDiffLength       *uint64              `json:"state_diff_length,omitempty"`
}

// https://github.com/starkware-libs/starknet-specs/blob/cce1563eff702c87590bad3a48382d2febf1f7d9/api/starknet_api_openrpc.json#L1794
type BlockWithTxs struct {
	Status rpcv9.BlockStatus `json:"status,omitempty"`
	BlockHeader
	Transactions []*Transaction `json:"transactions"`
}

// https://github.com/starkware-libs/starknet-specs/blob/cce1563eff702c87590bad3a48382d2febf1f7d9/api/starknet_api_openrpc.json#L1769
type BlockWithTxHashes struct {
	Status rpcv9.BlockStatus `json:"status,omitempty"`
	BlockHeader
	TxnHashes []*felt.Felt `json:"transactions"`
}

// TransactionWithReceipt represents a transaction with its receipt
type TransactionWithReceipt struct {
	Transaction *Transaction              `json:"transaction"`
	Receipt     *rpcv9.TransactionReceipt `json:"receipt"`
}

// https://github.com/starkware-libs/starknet-specs/blob/cce1563eff702c87590bad3a48382d2febf1f7d9/api/starknet_api_openrpc.json#L1819
type BlockWithReceipts struct {
	Status rpcv9.BlockStatus `json:"status,omitempty"`
	BlockHeader
	Transactions []TransactionWithReceipt `json:"transactions"`
}

// BlockWithTxHashes returns the block information with transaction hashes given a block ID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/cce1563eff702c87590bad3a48382d2febf1f7d9/api/starknet_api_openrpc.json#L25
func (h *Handler) BlockWithTxHashes(id *rpcv9.BlockID) (*BlockWithTxHashes, *jsonrpc.Error) {
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

	var commitments *core.BlockCommitments
	var stateDiff *core.StateDiff
	if block.Hash != nil {
		var err error
		commitments, stateDiff, err = h.getCommitmentsAndStateDiff(block.Number)
		if err != nil {
			return nil, rpccore.ErrInternal.CloneWithData(err)
		}
	}

	return &BlockWithTxHashes{
		Status:      status,
		BlockHeader: AdaptBlockHeader(block.Header, commitments, stateDiff),
		TxnHashes:   txnHashes,
	}, nil
}

// BlockWithTxHashes returns the block information with transaction receipts given a block ID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/cce1563eff702c87590bad3a48382d2febf1f7d9/api/starknet_api_openrpc.json#L99
func (h *Handler) BlockWithReceipts(
	id *rpcv9.BlockID,
	responseFlags ResponseFlags,
) (*BlockWithReceipts, *jsonrpc.Error) {
	includeProofFacts := responseFlags.IncludeProofFacts

	block, rpcErr := h.blockByID(id)
	if rpcErr != nil {
		return nil, rpcErr
	}

	blockStatus, rpcErr := h.blockStatus(id, block)
	if rpcErr != nil {
		return nil, rpcErr
	}

	var finalityStatus rpcv9.TxnFinalityStatus
	switch s := blockStatus; s {
	case rpcv9.BlockAcceptedL1:
		finalityStatus = rpcv9.TxnAcceptedOnL1
	case rpcv9.BlockAcceptedL2:
		finalityStatus = rpcv9.TxnAcceptedOnL2
	case rpcv9.BlockPreConfirmed:
		finalityStatus = rpcv9.TxnPreConfirmed
		// legacy pending block
		if block.ParentHash != nil {
			finalityStatus = rpcv9.TxnAcceptedOnL2
		}
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
			Receipt: rpcv9.AdaptReceipt(r, txn, finalityStatus),
		}
	}

	var commitments *core.BlockCommitments
	var stateDiff *core.StateDiff
	var err error
	if block.Hash != nil {
		commitments, stateDiff, err = h.getCommitmentsAndStateDiff(block.Number)
		if err != nil {
			return nil, rpccore.ErrInternal.CloneWithData(err)
		}
	}

	return &BlockWithReceipts{
		Status:       blockStatus,
		BlockHeader:  AdaptBlockHeader(block.Header, commitments, stateDiff),
		Transactions: txsWithReceipts,
	}, nil
}

type ResponseFlags struct {
	IncludeProofFacts bool `json:"include_proof_facts"`
}

func (r *ResponseFlags) UnmarshalJSON(data []byte) error {
	var flags []string
	if err := json.Unmarshal(data, &flags); err != nil {
		return err
	}

	r.IncludeProofFacts = false

	for _, flag := range flags {
		switch flag {
		case "INCLUDE_PROOF_FACTS":
			r.IncludeProofFacts = true
		default:
			return fmt.Errorf("unknown flag: %s", flag)
		}
	}

	return nil
}

// BlockWithTxs returns the block information with full transactions given a block ID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/cce1563eff702c87590bad3a48382d2febf1f7d9/api/starknet_api_openrpc.json#L62
func (h *Handler) BlockWithTxs(
	blockID *rpcv9.BlockID,
	responseFlags ResponseFlags,
) (*BlockWithTxs, *jsonrpc.Error) {
	includeProofFacts := responseFlags.IncludeProofFacts

	block, rpcErr := h.blockByID(blockID)
	if rpcErr != nil {
		return nil, rpcErr
	}

	txs := make([]*Transaction, len(block.Transactions))
	for index, txn := range block.Transactions {
		adaptedTx := AdaptTransaction(txn, includeProofFacts)
		txs[index] = &adaptedTx
	}

	status, rpcErr := h.blockStatus(blockID, block)
	if rpcErr != nil {
		return nil, rpcErr
	}

	var commitments *core.BlockCommitments
	var stateDiff *core.StateDiff
	var err error
	if block.Hash != nil {
		commitments, stateDiff, err = h.getCommitmentsAndStateDiff(block.Number)
		if err != nil {
			return nil, rpccore.ErrInternal.CloneWithData(err)
		}
	}

	return &BlockWithTxs{
		Status:       status,
		BlockHeader:  AdaptBlockHeader(block.Header, commitments, stateDiff),
		Transactions: txs,
	}, nil
}

func (h *Handler) blockStatus(
	id *rpcv9.BlockID,
	block *core.Block,
) (rpcv9.BlockStatus, *jsonrpc.Error) {
	l1H, jsonErr := h.l1Head()
	if jsonErr != nil {
		return 0, jsonErr
	}

	status := rpcv9.BlockAcceptedL2
	if id.IsPreConfirmed() {
		status = rpcv9.BlockPreConfirmed
	} else if isL1Verified(block.Number, l1H) {
		status = rpcv9.BlockAcceptedL1
	}

	return status, nil
}

func AdaptBlockHeader(
	header *core.Header,
	commitments *core.BlockCommitments,
	stateDiff *core.StateDiff,
) BlockHeader {
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
			InWei: nilToOne(header.L1DataGasPrice.PriceInWei),
			InFri: nilToOne(header.L1DataGasPrice.PriceInFri),
		}
	} else {
		l1DataGasPrice = rpcv6.ResourcePrice{
			InWei: &felt.One,
			InFri: &felt.One,
		}
	}

	var l2GasPrice rpcv6.ResourcePrice
	if header.L2GasPrice != nil {
		l2GasPrice = rpcv6.ResourcePrice{
			InWei: nilToOne(header.L2GasPrice.PriceInWei),
			InFri: nilToOne(header.L2GasPrice.PriceInFri),
		}
	} else {
		l2GasPrice = rpcv6.ResourcePrice{
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
		L1GasPrice: &rpcv6.ResourcePrice{
			InWei: nilToOne(header.L1GasPriceETH),
			InFri: nilToOne(header.L1GasPriceSTRK),
		},
		L1DataGasPrice:  &l1DataGasPrice,
		L1DAMode:        &l1DAMode,
		StarknetVersion: header.ProtocolVersion,
		L2GasPrice:      &l2GasPrice,
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

		// Populate state diff length
		stateDiffLength := stateDiff.Length()
		blockHeader.StateDiffLength = &stateDiffLength
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
