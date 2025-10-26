package vm

import (
	"encoding/json"
	"errors"
	"slices"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
)

type StateDiff struct {
	StorageDiffs              []StorageDiff           `json:"storage_diffs"`
	Nonces                    []Nonce                 `json:"nonces"`
	DeployedContracts         []DeployedContract      `json:"deployed_contracts"`
	DeprecatedDeclaredClasses []*felt.Felt            `json:"deprecated_declared_classes"`
	DeclaredClasses           []DeclaredClass         `json:"declared_classes"`
	ReplacedClasses           []ReplacedClass         `json:"replaced_classes"`
	MigratedCompiledClasses   []MigratedCompiledClass `json:"migrated_compiled_classes"`
}

type MigratedCompiledClass struct {
	ClassHash         felt.SierraClassHash `json:"class_hash"`
	CompiledClassHash felt.CasmClassHash   `json:"compiled_class_hash"`
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

func (t TransactionType) String() string {
	switch t {
	case TxnDeclare:
		return "DECLARE"
	case TxnDeploy:
		return "DEPLOY"
	case TxnDeployAccount:
		return "DEPLOY_ACCOUNT"
	case TxnInvoke:
		return "INVOKE"
	case TxnL1Handler:
		return "L1_HANDLER"
	default:
		return "<unknown>"
	}
}

func (t TransactionType) MarshalText() ([]byte, error) {
	return []byte(t.String()), nil
}

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
	FunctionInvocation    *ExecuteInvocation  `json:"function_invocation,omitempty"`
	StateDiff             *StateDiff          `json:"state_diff,omitempty"`
	ExecutionResources    *ExecutionResources `json:"execution_resources,omitempty"`
}

// Todo: this was deleted during RP refactoring. However, the builder pkg needs it.
// Think about if we can handle this more elegantly.
func (t *TransactionTrace) TotalExecutionResources() *ExecutionResources {
	total := new(ExecutionResources)
	for _, invocation := range t.allInvocations() {
		r := invocation.ExecutionResources
		if r == nil {
			continue
		}
		total.Pedersen += r.Pedersen
		total.RangeCheck += r.RangeCheck
		total.Bitwise += r.Bitwise
		total.Ecdsa += r.Ecdsa
		total.EcOp += r.EcOp
		total.Keccak += r.Keccak
		total.Poseidon += r.Poseidon
		total.SegmentArena += r.SegmentArena
		total.MemoryHoles += r.MemoryHoles
		total.Steps += r.Steps
	}
	return total
}

type TransactionReceipt struct {
	Fee   *felt.Felt
	Gas   GasConsumed
	DAGas DataAvailability
}

func (t *TransactionTrace) allInvocations() []*FunctionInvocation {
	var executeInvocation *FunctionInvocation
	if t.ExecuteInvocation != nil {
		executeInvocation = t.ExecuteInvocation.FunctionInvocation
	}
	var functionInvocation *FunctionInvocation
	if t.FunctionInvocation != nil {
		functionInvocation = t.FunctionInvocation.FunctionInvocation
	}
	return slices.DeleteFunc([]*FunctionInvocation{
		t.ConstructorInvocation,
		t.ValidateInvocation,
		t.FeeTransferInvocation,
		executeInvocation,
		functionInvocation,
	}, func(i *FunctionInvocation) bool { return i == nil })
}

func (t *TransactionTrace) RevertReason() string {
	if t.ExecuteInvocation == nil {
		return ""
	}
	return t.ExecuteInvocation.RevertReason
}

func (t *TransactionTrace) AllEvents() []OrderedEvent {
	events := make([]OrderedEvent, 0)
	globalOrder := 0

	addEvents := func(invocation *FunctionInvocation) {
		if invocation != nil {
			allEvents := invocation.allEvents()
			for _, event := range allEvents {
				event.Order += uint64(globalOrder)
				events = append(events, event)
			}
			globalOrder += len(allEvents)
		}
	}

	if t.Type == TxnDeployAccount {
		if t.ExecuteInvocation != nil {
			addEvents(t.ExecuteInvocation.FunctionInvocation)
		}
		addEvents(t.ConstructorInvocation)
		addEvents(t.ValidateInvocation)
	} else {
		addEvents(t.ValidateInvocation)
		if t.ExecuteInvocation != nil {
			addEvents(t.ExecuteInvocation.FunctionInvocation)
		}
		addEvents(t.ConstructorInvocation)
	}
	addEvents(t.FeeTransferInvocation)
	if t.FunctionInvocation != nil {
		addEvents(t.FunctionInvocation.FunctionInvocation)
	}

	return events
}

func (t *TransactionTrace) AllMessages() []OrderedL2toL1Message {
	messages := make([]OrderedL2toL1Message, 0)
	for _, invocation := range t.allInvocations() {
		messages = append(messages, invocation.allMessages()...)
	}
	return messages
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

func (invocation *FunctionInvocation) allEvents() []OrderedEvent {
	events := make([]OrderedEvent, 0)
	for i := range invocation.Calls {
		events = append(events, invocation.Calls[i].allEvents()...)
	}
	return append(events, utils.Map(invocation.Events, func(e OrderedEvent) OrderedEvent {
		e.From = &invocation.ContractAddress
		return e
	})...)
}

func (invocation *FunctionInvocation) allMessages() []OrderedL2toL1Message {
	messages := make([]OrderedL2toL1Message, 0)
	for i := range invocation.Calls {
		messages = append(messages, invocation.Calls[i].allMessages()...)
	}
	return append(messages, utils.Map(invocation.Messages, func(e OrderedL2toL1Message) OrderedL2toL1Message {
		e.From = &invocation.ContractAddress
		return e
	})...)
}

type ExecuteInvocation struct {
	RevertReason        string `json:"revert_reason"`
	*FunctionInvocation `json:",omitempty"`
}

func (e ExecuteInvocation) MarshalJSON() ([]byte, error) {
	if e.FunctionInvocation != nil {
		return json.Marshal(e.FunctionInvocation)
	}
	type alias ExecuteInvocation
	return json.Marshal(alias(e))
}

type OrderedEvent struct {
	Order uint64       `json:"order"`
	From  *felt.Felt   `json:"from_address,omitempty"`
	Keys  []*felt.Felt `json:"keys"`
	Data  []*felt.Felt `json:"data"`
}

type OrderedL2toL1Message struct {
	Order   uint64        `json:"order"`
	From    *felt.Felt    `json:"from_address,omitempty"`
	To      *felt.Address `json:"to_address"`
	Payload []*felt.Felt  `json:"payload"`
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

type GasConsumed struct {
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

func NewDataAvailability(gasConsumed, dataGasConsumed *felt.Felt, mode core.L1DAMode) DataAvailability {
	da := DataAvailability{}

	switch mode {
	case core.Calldata:
		da.L1Gas = gasConsumed.Uint64()
	case core.Blob:
		da.L1DataGas = dataGasConsumed.Uint64()
	}

	return da
}
