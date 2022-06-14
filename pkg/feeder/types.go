package feeder

// notest
import (
	feeder "github.com/NethermindEth/juno/pkg/feeder/abi"
	"github.com/NethermindEth/juno/pkg/feeder/types"
	"github.com/NethermindEth/juno/pkg/rpc"
)

type (
	// TxnType represents the type of each transaction.
	TxnType int
	// ChainID represent the chain identifier.
	ChainID string
	// XXX: Document.
	// TODO: replace with real hash representation.
	Hash string
)

// Represent the types of transactions
const (
	Deploy TxnType = iota
	InitializeBlockInfo
	Invoke
)

// Represent the identifiers of the
const (
	Mainnet ChainID = "MAINNET"
	Testnet ChainID = "Goerli"
)

// ContractAddresses represent the response for Starknet contract
// address details.
type ContractAddresses struct {
	GpsStatementVerifier string `json:"GpsStatementVerifier"`
	Starknet             string `json:"Starknet"`
}

// StarknetGeneralConfig represent StarkNet general configuration.
type StarknetGeneralConfig struct {
	ChainID                             ChainID `json:"chain_id"`
	ContractStorageCommitmentTreeHeight int64   `json:"contract_storage_commitment_tree_height"`
	GlobalStateCommitmentTreeHeight     int64   `json:"global_state_commitment_tree_height"`
	InvokeTxMaxNSteps                   int64   `json:"invoke_tx_max_n_steps"`
	// StarkNet sequencer address.
	SequencerAddress int64 `json:"sequencer_address"`
	// Height of Patricia tree of the transaction commitment in a block.
	TxCommitmentTreeHeight int `json:"tx_commitment_tree_height"`
	// Height of Patricia tree of the event commitment in a block.
	EventCommitmentTreeHeight int `json:"event_commitment_tree_height"`
	// A mapping from a Cairo usage resource to its coefficient in this
	// transaction fee calculation.
	CairoUsageResourceFeeWeights map[string]float64 `json:"cairo_usage_resource_fee_weights"`
}

// Transaction StarkNet transaction base interface
type Transaction interface {
	TransactionType() TxnType
	CalculateHash(config StarknetGeneralConfig) Hash
}

// InvokeFunction represents a transaction in the StarkNet network that
// is an invocation of a Cairo contract function.
type InvokeFunction struct {
	CallerAddress      string   `json:"caller_address"`
	ContractAddress    string   `json:"contract_address"`
	CodeAddress        string   `json:"code_address"`
	Calldata           []string `json:"calldata"`
	CallType           string   `json:"call_type"`
	ClassHash          string   `json:"class_hash"`
	EntryPointSelector string   `json:"entry_point_selector"`
	Selector           string   `json:"selector"`
	EntryPointType     string   `json:"entry_point_type"`
	Result             []string `json:"result"`
	// Additional information given by the caller that represents the
	// signature of the transaction. The exact way this field is handled
	// is defined by the called contract's function, like calldata.
	ExecutionResources `json:"execution_resources"`
	// The transaction is not valid if its version is lower than the current version,
	// defined by the SN OS.
	Version        int              `json:"version"`
	Signature      []int            `json:"signature"`
	InternallCalls []InvokeFunction `json:"internall_calls"`
	Events         []Event          `json:"events"`
	Messages       []string         `json:"messages"`
	// The maximal fee to be paid in Wei for executing invoked function.
	MaxFee string `json:"max_fee"`
}

// TransactionType returns the TxnType related to InvokeFunction
func (i InvokeFunction) TransactionType() TxnType {
	return Invoke
}

// XXX: Document.
// CalculateHash returns the hash of the tra
func (i InvokeFunction) CalculateHash(config StarknetGeneralConfig) Hash {
	// TODO: implement this
	return Hash(config.ChainID)
}

// TxnSpecificInfo represent a StarkNet transaction information.
type TxnSpecificInfo struct {
	ContractAddress    string   `json:"contract_address"`
	EntryPointSelector string   `json:"entry_point_selector"`
	EntryPointType     string   `json:"entry_point_type"`
	Calldata           []string `json:"calldata"`
	Signature          []string `json:"signature"`
	TransactionHash    string   `json:"transaction_hash"`
	MaxFee             string   `json:"max_fee"`
	Type               string   `json:"type"`
}

// L1ToL2Message Represents a StarkNet L1-to-L2 message.
type L1ToL2Message struct {
	FromAddress string   `json:"from_address"`
	ToAddress   string   `json:"to_address"`
	Selector    string   `json:"selector"`
	Payload     []string `json:"payload"`
	Nonce       string   `json:"nonce"`
}

// L2ToL1Message Represents a StarkNet L2-to-L1 message.
type L2ToL1Message struct {
	FromAddress string   `json:"from_address"`
	ToAddress   string   `json:"to_address,omitemtpy"`
	Payload     []string `json:"payload,omitemtpy"`
}

// Event Represents a StarkNet event; contains all the fields that will
// be included in the block hash.
type Event struct {
	FromAddress string   `json:"from_address"`
	Keys        []string `json:"keys"`
	Data        []string `json:"data"`
}

// ExecutionResources Indicates how many steps the program should run,
// how many memory cells are used from each builtin, and how many holes
// there are in the memory address space.
type ExecutionResources struct {
	NSteps                 int64            `json:"n_steps"`
	BuiltinInstanceCounter map[string]int64 `json:"builtin_instance_counter"`
	NMemoryHoles           int64            `json:"n_memory_holes"`
}

// TransactionExecution Represents a receipt of an executed transaction.
type TransactionExecution struct {
	// The index of the transaction within the block.
	TransactionIndex int64 `json:"transaction_index"`
	// A unique identifier of the transaction.
	TransactionHash string `json:"transaction_hash"`
	// L2-to-L1 messages.
	L2ToL1Messages []L2ToL1Message `json:"l2_to_l1_messages"`
	// L1-to-L2 messages.
	L1ToL2Message `json:"l1_to_l2_consumed_message"`
	// Events emitted during the execution of the transaction.
	Events []Event `json:"events"`
	// The resources needed by the transaction.
	ExecutionResources `json:"execution_resources"`
	// Fee paid for executing the transaction.
	ActualFee string `json:"actual_fee"`
}

// StarknetBlock Represents a response StarkNet block.
type StarknetBlock struct {
	BlockHash           string                 `json:"block_hash"`
	ParentBlockHash     string                 `json:"parent_block_hash"`
	BlockNumber         types.BlockNumber      `json:"block_number"`
	GasPrice            string                 `json:"gas_price"`
	SequencerAddress    string                 `json:"sequencer_address"`
	StateRoot           string                 `json:"state_root"`
	Status              rpc.BlockStatus        `json:"status"`
	Transactions        []TxnSpecificInfo      `json:"transactions"`
	Timestamp           int64                  `json:"timestamp"`
	TransactionReceipts []TransactionExecution `json:"transaction_receipts"`
}

// struct to store Storage info
type StorageInfo string

// struct for code type
type CodeInfo struct {
	Bytecode []string   `json:"bytecode"`
	Abi      feeder.Abi `json:"abi"`
}

// TransactionFailureReason store reason of failure in transactions.
type TransactionFailureReason struct {
	TxID     int64  `json:"tx_id"`
	Code     string `json:"code"`
	ErrorMsg string `json:"error_message"`
}

// type TxnStatus string

// TransactionInfo store all the transaction Information
type TransactionInfo struct {
	// // Block information that Transaction occured in
	TransactionInBlockInfo // fix: Was not fetching values correctly prior
	// Transaction Specific Information
	Transaction TxnSpecificInfo `json:"transaction"`
}

// TransactionInBlockInfo represents the information regarding a
// transaction that appears in a block.
type TransactionInBlockInfo struct {
	// The reason for the transaction failure, if applicable.
	TransactionFailureReason `json:"transaction_failure_reason"`
	TransactionStatus
	// The sequence number of the block corresponding to block_hash, which
	// is the number of blocks prior to it in the active chain.
	BlockNumber int64 `json:"block_number"`
	//	The index of the transaction within the block corresponding to
	// block_hash.
	TransactionIndex int64 `json:"transaction_index"`
}

type TransactionStatus struct {
	// tx_status for get_transaction_status
	TxStatus string `json:"tx_status"`
	// status for other calls.
	Status    string `json:"status"`
	BlockHash string `json:"block_hash"`
}

// TransactionReceipt represents a receipt of a StarkNet transaction;
// i.e., the information regarding its execution and the block it
// appears in.
type TransactionReceipt struct {
	TransactionInBlockInfo
	TransactionExecution
}

type TransactionTrace struct {
	InvokeFunction `json:"function_invocation"`
	Signature      []string `json:"signature"`
}

// KV represents a key-value pair.
type KV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// StateUpdateResponse represents the response of a StarkNet state
// update.
type StateUpdateResponse struct {
	BlockHash string `json:"block_hash"`
	NewRoot   string `json:"new_root"`
	OldRoot   string `json:"old_root"`
	StateDiff struct {
		DeployedContracts []struct {
			Address      string `json:"address"`
			ContractHash string `json:"contract_hash"`
		} `json:"deployed_contracts"`
		StorageDiffs map[string][]KV `json:"storage_diffs"`
	} `json:"state_diff"`
}
