package starknet_client

import cmd "github.com/NethermindEth/juno/cmd/starknet"

type BlockStatus string

// BlockStatus const
const (
	Pending      BlockStatus = "PENDING"
	Proven                   = "PROVEN"
	AcceptedOnL2             = "ACCEPTED_ON_L2"
	AcceptedOnL1             = "ACCEPTED_ON_L1"
	Rejected                 = "REJECTED"
)

// ResponseError represent the possible errors of StarkNet RPC API
type ResponseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

var (
	FailedToReceiveTxn     = ResponseError{1, "Failed to write transaction"}
	ContractNotFound       = ResponseError{20, "Contract not found"}
	InvalidMessageSelector = ResponseError{21, "Invalid message selector"}
	InvalidCallData        = ResponseError{22, "Invalid call data"}
	InvalidStorageKey      = ResponseError{23, "Invalid storage key"}
	InvalidBlockHash       = ResponseError{24, "Invalid block hash"}
	InvalidTxnHash         = ResponseError{25, "Invalid transaction hash"}
	InvalidBlockNumber     = ResponseError{26, "Invalid block number"}
	ContractError          = ResponseError{40, "Contract error"}
)

type Transactions struct {
}

type BlockResponse struct {
	// A field element of 251 bits. Represented as up to 64 hex digits
	BlockHash string `json:"block_hash"`
	// The hash of this block's parent
	ParentHash string `json:"parent_hash"`
	// The block number (its height)
	BlockNumber uint64 `json:"block_number"`
	// The status of the block
	Status BlockStatus `json:"status"`
	// The identity of the sequencer submitting this block
	Sequencer string `json:"sequencer"`
	// The new global state root
	NewRoot cmd.Felt `json:"new_root"`
	// The previous global state root
	OldRoot string `json:"old_root"`
	// When the block was accepted on L1. Formatted as...
	AcceptedTime uint64 `json:"accepted_time"`
	// Transactions in the Block
	Transactions []Transactions `json:"transactions"`
}

// BlockHash is The hash (id) of the requested block, or a block tag
type BlockHash struct {
	// A field element of 251 bits. Represented as up to 64 hex digits
	Hash string
	// A tag specifying a dynamic reference to a block
	Tag string
}

// BlockNumber is the number of the requested block, or a block tag
type BlockNumber struct {
	// The block's number
	Number string
	// A tag specifying a dynamic reference to a block
	Tag string
}

type RequestedScope string

// BlockStatus const
const (
	TxnHash            RequestedScope = "TXN_HASH"
	FullTxns                          = "FULL_TXNS"
	FullTxnAndReceipts                = "FULL_TXN_AND_RECEIPTS"
)

type StarkNetHash struct{}
