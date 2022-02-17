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
