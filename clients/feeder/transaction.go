package feeder

import (
	"github.com/NethermindEth/juno/core/felt"
)

// Transaction object returned by the feeder in JSON format for multiple endpoints
type Transaction struct {
	Hash                *felt.Felt    `json:"transaction_hash,omitempty" copier:"must,nopanic"`
	Version             *felt.Felt    `json:"version,omitempty"`
	ContractAddress     *felt.Felt    `json:"contract_address,omitempty"`
	ContractAddressSalt *felt.Felt    `json:"contract_address_salt,omitempty"`
	ClassHash           *felt.Felt    `json:"class_hash,omitempty"`
	ConstructorCallData *[]*felt.Felt `json:"constructor_calldata,omitempty"`
	Type                string        `json:"type,omitempty"`
	SenderAddress       *felt.Felt    `json:"sender_address,omitempty"`
	MaxFee              *felt.Felt    `json:"max_fee,omitempty"`
	Signature           *[]*felt.Felt `json:"signature,omitempty"`
	CallData            *[]*felt.Felt `json:"calldata,omitempty"`
	EntryPointSelector  *felt.Felt    `json:"entry_point_selector,omitempty"`
	Nonce               *felt.Felt    `json:"nonce,omitempty"`
	CompiledClassHash   *felt.Felt    `json:"compiled_class_hash,omitempty"`
}

type TransactionStatus struct {
	Status           string       `json:"status"`
	BlockHash        *felt.Felt   `json:"block_hash"`
	BlockNumber      uint64       `json:"block_number"`
	TransactionIndex uint64       `json:"transaction_index"`
	Transaction      *Transaction `json:"transaction"`
}

type Event struct {
	From *felt.Felt   `json:"from_address"`
	Data []*felt.Felt `json:"data"`
	Keys []*felt.Felt `json:"keys"`
}

type L1ToL2Message struct {
	From     string       `json:"from_address"`
	Payload  []*felt.Felt `json:"payload"`
	Selector *felt.Felt   `json:"selector"`
	To       *felt.Felt   `json:"to_address"`
	Nonce    *felt.Felt   `json:"nonce"`
}

type L2ToL1Message struct {
	From    *felt.Felt   `json:"from_address"`
	Payload []*felt.Felt `json:"payload"`
	To      string       `json:"to_address"`
}

type ExecutionResources struct {
	Steps                  uint64                 `json:"n_steps"`
	BuiltinInstanceCounter BuiltinInstanceCounter `json:"builtin_instance_counter"`
	MemoryHoles            uint64                 `json:"n_memory_holes"`
}

type BuiltinInstanceCounter struct {
	Pedersen   uint64 `json:"pedersen_builtin"`
	RangeCheck uint64 `json:"range_check_builtin"`
	Bitwise    uint64 `json:"bitwise_builtin"`
	Output     uint64 `json:"output_builtin"`
	Ecsda      uint64 `json:"ecdsa_builtin"`
	EcOp       uint64 `json:"ec_op_builtin"`
}

type TransactionReceipt struct {
	ActualFee          *felt.Felt          `json:"actual_fee"`
	Events             []*Event            `json:"events"`
	ExecutionResources *ExecutionResources `json:"execution_resources"`
	L1ToL2Message      *L1ToL2Message      `json:"l1_to_l2_consumed_message"`
	L2ToL1Message      []*L2ToL1Message    `json:"l2_to_l1_messages"`
	TransactionHash    *felt.Felt          `json:"transaction_hash"`
	TransactionIndex   uint64              `json:"transaction_index"`
}
