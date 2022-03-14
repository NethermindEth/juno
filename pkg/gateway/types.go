package gateway

type TxnType int
type StarknetChainId string

// Hash TODO replace with real hash representation
type Hash string

const (
	DEPLOYType TxnType = iota
	InitializeBlockInfoType
	InvokeFunctionType
)

const (
	MainnetChainId StarknetChainId = "MAINNET"
	TesNetChainId  StarknetChainId = "Goerli"
)

// StarknetGeneralConfig represent StarkNet General configuration
type StarknetGeneralConfig struct {
	ChainId                             StarknetChainId `json:"chain_id"`
	ContractStorageCommitmentTreeHeight int64           `json:"contract_storage_commitment_tree_height"`
	GlobalStateCommitmentTreeHeight     int64           `json:"global_state_commitment_tree_height"`
	InvokeTxMaxNSteps                   int64           `json:"invoke_tx_max_n_steps"`
	// StarkNet sequencer address.
	SequencerAddress int64 `json:"sequencer_address"`
	// Height of Patricia tree of the transaction commitment in a block.
	TxCommitmentTreeHeight int `json:"tx_commitment_tree_height"`
	// Height of Patricia tree of the event commitment in a block.
	EventCommitmentTreeHeight int `json:"event_commitment_tree_height"`
	// A mapping from a Cairo usage resource to its coefficient in this transaction fee calculation.
	CairoUsageResourceFeeWeights map[string]float64 `json:"cairo_usage_resource_fee_weights"`
}

// Transaction StarkNet transaction base interface
type Transaction interface {
	TransactionType() TxnType
	CalculateHash(config StarknetGeneralConfig) Hash
}

// InvokeFunction  Represents a transaction in the StarkNet network that is an invocation of a Cairo contract function.
type InvokeFunction struct {
	ContractAddress int `json:"contract_address"`
	// A field element that encodes the signature of the called function.
	EntryPointSelector int   `json:"entry_point_selector"`
	Calldata           []int `json:"calldata"`
	// Additional information given by the caller that represents the signature of the transaction.
	// The exact way this field is handled is defined by the called contract's function, like
	// calldata.
	Signature []int `json:"signature"`
}

func (i InvokeFunction) TransactionType() TxnType {
	return InvokeFunctionType
}
func (i InvokeFunction) CalculateHash(config StarknetGeneralConfig) Hash {
	// TODO implement this
	return Hash(config.ChainId)
}
