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

// Felt represent aN field element. Represented as up to 63 hex digits and leading 4 bits zeroed.
type Felt string

// BlockNumber The block's number (its height)
type BlockNumber uint64

// ChainID StarkNet chain id, given in hex representation.
type ChainID string

// ProtocolVersion StarkNet protocol version, given in hex representation.
type ProtocolVersion string

type BlockNumberOrTag struct {
	BlockNumber
	BlockTag
}

// BlockHashOrTag The hash (id) of the requested block or a block tag, for the block referencing the state or call the transaction on.
type BlockHashOrTag struct {
	BlockHash
	BlockTag
}

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

const (
	TxnStatusUnknown      TxnStatus = "UNKNOWN"
	TxnStatusReceived               = "RECEIVED"
	TxnStatusPending                = "PENDING"
	TxnStatusAcceptedOnL2           = "ACCEPTED_ON_L2"
	TxnStatusAcceptedOnL1           = "ACCEPTED_ON_L1"
	TxnStatusRejected               = "REJECTED"
)

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
