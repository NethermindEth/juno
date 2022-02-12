package cmd

// FunctionCall represents the details of the function call
type FunctionCall struct {
	// ContractAddress Is a field element of 251 bits. Represented as up to 64 hex digits
	ContractAddress string `json:"contract_address"`
	// EntryPointSelector Is a field element of 251 bits. Represented as up to 64 hex digits
	EntryPointSelector string `json:"entry_point_selector"`
	// CallData are the parameters passed to the function
	CallData []string
}

// BlockHash Is a field element of 251 bits. Represented as up to 64 hex digits
type BlockHash string

// BlockTag Is a tag specifying a dynamic reference to a block
type BlockTag string

// BlockHashOrTag The hash (id) of the requested block or a block tag, for the block referencing the state or call the transaction on.
type BlockHashOrTag struct {
	BlockHash BlockHash `json:"block_hash"`
	BlockTag  BlockTag  `json:"block_tag"`
}

// RequestRPC Represent the calls a function in a contract and returns the return value.  Using this call will not create a transaction; hence, will not change the state
type RequestRPC struct {
	Request   FunctionCall `json:"request"`
	BlockHash BlockHash    `json:"block_hash"`
}

// ResultCall Is a field element of 251 bits. Represented as up to 64 hex digits
type ResultCall []string

type RequestedScope string

// BlockStatus const
const (
	TxnHash            RequestedScope = "TXN_HASH"
	FullTxns                          = "FULL_TXNS"
	FullTxnAndReceipts                = "FULL_TXN_AND_RECEIPTS"
)

type BlockStatus string

// BlockStatus const
const (
	Pending      BlockStatus = "PENDING"
	Proven                   = "PROVEN"
	AcceptedOnL2             = "ACCEPTED_ON_L2"
	AcceptedOnL1             = "ACCEPTED_ON_L1"
	Rejected                 = "REJECTED"
)

type Transactions struct{}
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
	NewRoot string `json:"new_root"`
	// The previous global state root
	OldRoot string `json:"old_root"`
	// When the block was accepted on L1. Formatted as...
	AcceptedTime uint64 `json:"accepted_time"`
	// Transactions in the Block
	Transactions []Transactions `json:"transactions"`
}
