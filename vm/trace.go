package vm

import (
	"errors"

	"github.com/NethermindEth/juno/core/felt"
)

type StateDiff struct {
	StorageDiffs              []StorageDiff      `json:"storage_diffs"`
	Nonces                    []Nonce            `json:"nonces"`
	DeployedContracts         []DeployedContract `json:"deployed_contracts"`
	DeprecatedDeclaredClasses []*felt.Felt       `json:"deprecated_declared_classes"`
	DeclaredClasses           []DeclaredClass    `json:"declared_classes"`
	ReplacedClasses           []ReplacedClass    `json:"replaced_classes"`
}

type Nonce struct {
	ContractAddress felt.Felt `json:"contract_address"`
	Nonce           felt.Felt `json:"nonce"`
}

type StorageDiff struct {
	Address        felt.Felt `json:"address"`
	StorageEntries []Entry   `json:"storage_entries"`
}

type Entry struct {
	Key   felt.Felt `json:"key"`
	Value felt.Felt `json:"value"`
}

type DeployedContract struct {
	Address   felt.Felt `json:"address"`
	ClassHash felt.Felt `json:"class_hash"`
}

type ReplacedClass struct {
	ContractAddress felt.Felt `json:"contract_address"`
	ClassHash       felt.Felt `json:"class_hash"`
}

type DeclaredClass struct {
	ClassHash         felt.Felt `json:"class_hash"`
	CompiledClassHash felt.Felt `json:"compiled_class_hash"`
}
type TransactionType uint8

const (
	Invalid TransactionType = iota
	TxnDeclare
	TxnDeploy
	TxnDeployAccount
	TxnInvoke
	TxnL1Handler
)

func (t *TransactionType) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case `"DECLARE"`:
		*t = TxnDeclare
	case `"DEPLOY"`:
		*t = TxnDeploy
	case `"DEPLOY_ACCOUNT"`:
		*t = TxnDeployAccount
	case `"INVOKE"`, `"INVOKE_FUNCTION"`:
		*t = TxnInvoke
	case `"L1_HANDLER"`:
		*t = TxnL1Handler
	default:
		return errors.New("unknown TransactionType")
	}
	return nil
}

type TransactionTrace struct {
	Type                  TransactionType     `json:"type,omitempty"`
	ValidateInvocation    *FunctionInvocation `json:"validate_invocation,omitempty"`
	ExecuteInvocation     *ExecuteInvocation  `json:"execute_invocation,omitempty"`
	FeeTransferInvocation *FunctionInvocation `json:"fee_transfer_invocation,omitempty"`
	ConstructorInvocation *FunctionInvocation `json:"constructor_invocation,omitempty"`
	FunctionInvocation    *FunctionInvocation `json:"function_invocation,omitempty"`
	StateDiff             *StateDiff          `json:"state_diff,omitempty"`
	ExecutionResources    *ExecutionResources `json:"execution_resources,omitempty"`
}

type FunctionInvocation struct {
	ContractAddress    felt.Felt              `json:"contract_address"`
	EntryPointSelector *felt.Felt             `json:"entry_point_selector,omitempty"`
	Calldata           []felt.Felt            `json:"calldata"`
	CallerAddress      felt.Felt              `json:"caller_address"`
	ClassHash          *felt.Felt             `json:"class_hash,omitempty"`
	EntryPointType     string                 `json:"entry_point_type,omitempty"`
	CallType           string                 `json:"call_type,omitempty"`
	Result             []felt.Felt            `json:"result"`
	Calls              []FunctionInvocation   `json:"calls"`
	Events             []OrderedEvent         `json:"events"`
	Messages           []OrderedL2toL1Message `json:"messages"`
	ExecutionResources *ExecutionResources    `json:"execution_resources,omitempty"`
	IsReverted         bool                   `json:"is_reverted,omitempty"`
}

type ExecuteInvocation struct {
	RevertReason        string `json:"revert_reason"`
	*FunctionInvocation `json:",omitempty"`
}

type OrderedEvent struct {
	Order uint64       `json:"order"`
	From  *felt.Felt   `json:"from_address,omitempty"`
	Keys  []*felt.Felt `json:"keys"`
	Data  []*felt.Felt `json:"data"`
}

type OrderedL2toL1Message struct {
	Order   uint64       `json:"order"`
	From    *felt.Felt   `json:"from_address,omitempty"`
	To      string       `json:"to_address"` // todo: make common.Address after fixing starknet-api EthAddress serialisation
	Payload []*felt.Felt `json:"payload"`
}

type ComputationResources struct {
	Steps        uint64 `json:"steps"`
	MemoryHoles  uint64 `json:"memory_holes,omitempty"`
	Pedersen     uint64 `json:"pedersen_builtin_applications,omitempty"`
	RangeCheck   uint64 `json:"range_check_builtin_applications,omitempty"`
	Bitwise      uint64 `json:"bitwise_builtin_applications,omitempty"`
	Ecdsa        uint64 `json:"ecdsa_builtin_applications,omitempty"`
	EcOp         uint64 `json:"ec_op_builtin_applications,omitempty"`
	Keccak       uint64 `json:"keccak_builtin_applications,omitempty"`
	Poseidon     uint64 `json:"poseidon_builtin_applications,omitempty"`
	SegmentArena uint64 `json:"segment_arena_builtin,omitempty"`
	AddMod       uint64 `json:"add_mod_builtin,omitempty"`
	MulMod       uint64 `json:"mul_mod_builtin,omitempty"`
	RangeCheck96 uint64 `json:"range_check_96_builtin,omitempty"`
	Output       uint64 `json:"output_builtin,omitempty"`
}

type DataAvailability struct {
	L1Gas     uint64 `json:"l1_gas"`
	L1DataGas uint64 `json:"l1_data_gas"`
}

type ExecutionResources struct {
	L1Gas     uint64 `json:"l1_gas"`
	L1DataGas uint64 `json:"l1_data_gas"`
	L2Gas     uint64 `json:"l2_gas"`

	ComputationResources
	DataAvailability *DataAvailability `json:"data_availability,omitempty"`
}
