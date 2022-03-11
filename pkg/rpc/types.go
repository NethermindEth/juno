package rpc

// BlockStatus const
const (
	TxnHashStatus      RequestedScope = "TXN_HASH"
	FullTxns           RequestedScope = "FULL_TXNS"
	FullTxnAndReceipts RequestedScope = "FULL_TXN_AND_RECEIPTS"
)

const (
	TxnStatusUnknown      TxnStatus = "UNKNOWN"
	TxnStatusReceived     TxnStatus = "RECEIVED"
	TxnStatusPending      TxnStatus = "PENDING"
	TxnStatusAcceptedOnL2 TxnStatus = "ACCEPTED_ON_L2"
	TxnStatusAcceptedOnL1 TxnStatus = "ACCEPTED_ON_L1"
	TxnStatusRejected     TxnStatus = "REJECTED"
)

const (
	Pending      BlockStatus = "PENDING"
	Proven       BlockStatus = "PROVEN"
	AcceptedOnL2 BlockStatus = "ACCEPTED_ON_L2"
	AcceptedOnL1 BlockStatus = "ACCEPTED_ON_L1"
	Rejected     BlockStatus = "REJECTED"
)

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

// FunctionCall represents the details of the function call
type FunctionCall struct {
	// ContractAddress Is a field element of 251 bits. Represented as up to 64 hex digits
	ContractAddress string `json:"contract_address"`
	// EntryPointSelector Is a field element of 251 bits. Represented as up to 64 hex digits
	EntryPointSelector string `json:"entry_point_selector"`
	// CallData are the parameters passed to the function
	CallData []string `json:"calldata"`
}

// BlockHash Is a field element of 251 bits. Represented as up to 64 hex digits
type BlockHash string

// BlockTag Is a tag specifying a dynamic reference to a block
type BlockTag string

// Felt represent aN field element. Represented as up to 63 hex digits and leading 4 bits zeroed.
type Felt string

// BlockNumber The block's number (its height)
type BlockNumber uint64

// ChainID StarkNet chain id, given in hex representation.
type ChainID string

// ProtocolVersion StarkNet protocol version, given in hex representation.
type ProtocolVersion string

type BlockNumberOrTag interface{}

// BlockHashOrTag The hash (id) of the requested block or a block tag, for the block referencing the state or call the transaction on.
type BlockHashOrTag string

// RequestRPC Represent the calls a function in a contract and returns the return value.  Using this call will not create a transaction; hence, will not change the state
type RequestRPC struct {
	Request   FunctionCall `json:"request"`
	BlockHash BlockHash    `json:"block_hash"`
}

// ResultCall Is a field element of 251 bits. Represented as up to 64 hex digits
type ResultCall []string

// BlockTransactionCount represent the number of transactions in the designated block
type BlockTransactionCount struct {
	TransactionCount int
}

// StateDiffItem represent a change in a single storage item
type StateDiffItem struct {
	// The contract address for which the state changed
	Address Felt `json:"address"`
	// The key of the changed value
	Key Felt `json:"key"`
	// THe new value applied to the given address
	Value Felt `json:"value"`
}

// ContractItem represent a new contract added as part of the new state
type ContractItem struct {
	// The address of the contract code
	Address Felt `json:"address"`
	// The hash of the contract code
	ContractHash Felt `json:"contractHash"`
}

// StateDiff represent the change in state applied in this block, given as a mapping of addresses to the new values and/or new contracts
type StateDiff struct {
	StorageDiffs []StateDiffItem `json:"storage_diffs"`
	Contracts    []ContractItem  `json:"contracts"`
}

type StateUpdate struct {
	BlockHash    BlockHash `json:"block_hash"`
	NewRoot      Felt      `json:"new_root"`
	OldRoot      Felt      `json:"old_root"`
	AcceptedTime uint64    `json:"accepted_time"`
	StateDiff    StateDiff `json:"state_diff"`
}

type Address Felt

// TxnHash The transaction hash, as assigned in StarkNet
type TxnHash Felt

// Txn Transaction
type Txn struct {
	// The function the transaction invokes
	FunctionCall
	// The hash identifying the transaction
	TxnHash TxnHash `json:"txn_hash"`
}

type TxnStatus string

// EthAddress represent an ethereum address
type EthAddress string

type MsgToL1 struct {
	// The target L1 address the message is sent to
	ToAddress Felt `json:"to_address"`
	// The Payload of the message
	Payload Felt `json:"payload"`
}

type MsgToL2 struct {
	// The originating L1 contract that sent the message
	FromAddress EthAddress `json:"from_address"`
	// The Payload of the message. The call data to the L1 handler
	Payload []Felt `json:"payload"`
}

// EventFilter represent an event filter/query
type EventFilter struct {
	FromBlock BlockNumber `json:"fromBlock"`
	ToBlock   BlockNumber `json:"toBlock"`
	Address   Address     `json:"address"`
	Keys      []Felt      `json:"keys"`
}

// EventContent represent the content of an Event
type EventContent struct {
	Keys []Felt `json:"keys"`
	Data []Felt `json:"data"`
}

// Event represent a StarkNet Event
type Event struct {
	EventContent
	FromAddress Address `json:"from_address"`
}

// EmittedEvent Represent Event information decorated with metadata on where it was emitted
type EmittedEvent struct {
	Event
	BlockHash       BlockHash `json:"block_hash"`
	TransactionHash TxnHash   `json:"transaction_hash"`
}

// TxnReceipt Receipt of the transaction
type TxnReceipt struct {
	TxnHash         TxnHash   `json:"txn_hash"`
	Status          TxnStatus `json:"status"`
	StatusData      string    `json:"status_data"`
	MessagesSent    []MsgToL1 `json:"messages_sent"`
	L1OriginMessage MsgToL2   `json:"l1_origin_message"`
	Events          Event     `json:"events"`
}

// CodeResult The code and ABI for the requested contract
type CodeResult struct {
	Bytecode []Felt `json:"bytecode"`
	// The ABI of the contract in JSON format. Uses the same structure as EVM ABI
	Abi string `json:"abi"`
}

// SyncStatus Returns an object about the sync status, or false if the node is not syncing
type SyncStatus struct {
	// The hash of the block from which the sync started
	StartingBlock BlockHash `json:"starting_block"`
	// The hash of the current block being synchronized
	CurrentBlock BlockHash `json:"current_block"`
	// The hash of the estimated the highest block to be synchronized
	HighestBlock BlockHash `json:"highest_block"`
}

// ResultPageRequest A request for a specific page of results
type ResultPageRequest struct {
	PageSize   uint64 `json:"page_size"`
	PageNumber uint64 `json:"page_number"`
}

type BlockStatus string

// ResponseError represent the possible errors of StarkNet RPC API
type ResponseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

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
	NewRoot Felt `json:"new_root"`
	// The previous global state root
	OldRoot string `json:"old_root"`
	// When the block was accepted on L1. Formatted as...
	AcceptedTime uint64 `json:"accepted_time"`
	// Transactions in the Block
	Transactions []Transactions `json:"transactions"`
}

type RequestedScope string

type StarkNetHash struct{}

// EventRequest represent allOf types.EventFilter and types.ResultPageRequest
type EventRequest struct {
	EventFilter
	ResultPageRequest
}

// EmittedEventArray represent a set of emmited events
type EmittedEventArray []EmittedEvent

// EventResponse represent the struct of the response of events
type EventResponse struct {
	EmittedEventArray
	PageNumber uint64 `json:"page_number"`
}
