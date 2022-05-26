package feeder

// notest
import (
	"github.com/NethermindEth/juno/internal/db/abi"
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
	ContractAddress int `json:"contract_address"`
	// A field element that encodes the signature of the called function.
	EntryPointSelector int   `json:"entry_point_selector"`
	Calldata           []int `json:"calldata"`
	// Additional information given by the caller that represents the
	// signature of the transaction. The exact way this field is handled
	// is defined by the called contract's function, like calldata.
	Signature []int `json:"signature"`
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
	Calldata            []string `json:"constructor_calldata"`
	ContractAddress     string   `json:"contract_address"`
	ContractAddressSalt string   `json:"contract_address_salt"`
	EntryPointSelector  string   `json:"entry_point_selector"`
	EntryPointType      string   `json:"entry_point_type"`
	Signature           []string `json:"signature"`
	TransactionHash     string   `json:"transaction_hash"`
	Type                string   `json:"type"`
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
	ToAddress   string   `json:"to_address"`
	Payload     []string `json:"payload"`
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
	// L1-to-L2 messages.
	L1ToL2ConsumedMessage L1ToL2Message `json:"l1_to_l2_consumed_message"`
	// L2-to-L1 messages.
	L2ToL1Messages []L2ToL1Message `json:"l2_to_l1_messages"`
	// Events emitted during the execution of the transaction.
	Events []Event `json:"events"`
	// The resources needed by the transaction.
	ExecutionResources ExecutionResources `json:"execution_resources"`
	ActualFee          string             `json:"actual_fee"`
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

//struct to store Storage info
type StorageInfo struct {
	Storage string `json:"storage"`
}

// //ABI Struct struct
// type Struct struct {
// 	Name string `json:"name"`
// 	Type string `json:"type"`
// 	Size string `json:"size"`
// }

// //ABI L1Handler Function struct type:l1_handler
// type L1Handler struct {
// 	Inputs          []Function `json:"inputs"`
// 	Name            string     `json:"name"`
// 	Type            string     `json:"type"`
// 	Outputs         []Function `json:"outputs"`
// 	Statemutability string     `json:"stateMutability"`
// }

// //ABI Function struct
// type Function struct {
// 	Inputs          []Function `json:"inputs"`
// 	Name            string     `json:"name"`
// 	Type            string     `json:"type"`
// 	Outputs         []Function `json:"outputs"`
// 	StateMutability string     `json:"stateMutability"`
// }

// //ABI struct
// type ABI struct {
// 	Functions   []Function
// 	Events      []Event
// 	Structs     []Struct
// 	L1Handlers  []L1Handler
// 	Constructor Function
// }

//struct for code type
type CodeInfo struct {
	Bytecode []string               `json:"bytecode"`
	Abi      abi.Abi                //`json:"abi"`
	X        map[string]interface{} `json:"-"` // Manual Handling of ABI feilds
}

// TransactionFailureReason store reason of failure in transactions.
type TransactionFailureReason struct {
	TxID     int64  `json:"tx_id"`
	Code     string `json:"code"`
	ErrorMsg string `json:"error_message"`
}

// TransactionInfo store all the transaction Inf
type TransactionInfo struct {
	// The status of a transaction, see TransactionStatus.
	Status rpc.TxnStatus `json:"status"`
	// The reason for the transaction failure, if applicable.
	TransactionFailureReason TransactionFailureReason `json:"transaction_failure_reason"`
	// The unique identifier of the block on the active chain containing
	// the transaction.
	BlockHash string `json:"block_hash"`
	// The sequence number of the block corresponding to block_hash, which
	// is the number of blocks prior to it in the active chain.
	BlockNumber int `json:"block_number"`
	// The index of the transaction within the block corresponding to
	// block_hash.
	TransactionIndex int64           `json:"transaction_index"`
	Transaction      TxnSpecificInfo `json:"transaction"`
}

// TransactionInBlockInfo represents the information regarding a
// transaction that appears in a block.
type TransactionInBlockInfo struct {
	// The status of a transaction, see TransactionStatus.
	Status rpc.TxnStatus
	// The reason for the transaction failure, if applicable.
	TransactionFailureReason TransactionFailureReason `json:"transaction_failure_reason"`
	// The unique identifier of the block on the active chain containing
	// the transaction.
	BlockHash string `json:"block_hash"`
	// The sequence number of the block corresponding to block_hash, which
	// is the number of blocks prior to it in the active chain.
	BlockNumber string `json:"block_number"`
	//	The index of the transaction within the block corresponding to
	// block_hash.
	TransactionIndex int64 `json:"transaction_index"`
}

// TransactionReceipt represents a receipt of a StarkNet transaction;
// i.e., the information regarding its execution and the block it
// appears in.
type TransactionReceipt struct {
	TransactionExecution
	TransactionInBlockInfo
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
