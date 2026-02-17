package starknet

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
)

type ExecutionStatus uint8

const (
	Succeeded ExecutionStatus = iota + 1
	Reverted
	Rejected
)

func (es *ExecutionStatus) UnmarshalText(data []byte) error {
	switch str := string(data); str {
	case "SUCCEEDED":
		*es = Succeeded
	case "REVERTED":
		*es = Reverted
	case "REJECTED":
		*es = Rejected
	default:
		return fmt.Errorf("unknown ExecutionStatus %q", str)
	}
	return nil
}

type FinalityStatus uint8

const (
	AcceptedOnL2 FinalityStatus = iota + 1
	AcceptedOnL1
	NotReceived
	Received
	PreConfirmed
	Candidate
)

func (fs *FinalityStatus) UnmarshalText(data []byte) error {
	switch str := string(data); str {
	case "ACCEPTED_ON_L2":
		*fs = AcceptedOnL2
	case "ACCEPTED_ON_L1":
		*fs = AcceptedOnL1
	case "NOT_RECEIVED":
		*fs = NotReceived
	case "RECEIVED":
		*fs = Received
	case "PRE_CONFIRMED":
		*fs = PreConfirmed
	case "CANDIDATE":
		*fs = Candidate
	default:
		return fmt.Errorf("unknown FinalityStatus %q", str)
	}
	return nil
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
		return "INVOKE_FUNCTION"
	case TxnL1Handler:
		return "L1_HANDLER"
	default:
		return "<unknown>"
	}
}

func (t TransactionType) MarshalText() ([]byte, error) {
	return []byte(t.String()), nil
}

func (t *TransactionType) UnmarshalText(data []byte) error {
	switch str := string(data); str {
	case "DECLARE":
		*t = TxnDeclare
	case "DEPLOY":
		*t = TxnDeploy
	case "DEPLOY_ACCOUNT":
		*t = TxnDeployAccount
	case "INVOKE", "INVOKE_FUNCTION":
		*t = TxnInvoke
	case "L1_HANDLER":
		*t = TxnL1Handler
	default:
		return fmt.Errorf("unknown TransactionType %q", str)
	}
	return nil
}

type Resource uint32

const (
	ResourceL1Gas Resource = iota + 1
	ResourceL2Gas
	ResourceL1DataGas
)

func (r *Resource) UnmarshalJSON(data []byte) error {
	return r.UnmarshalText(bytes.Trim(data, `"`))
}

func (r *Resource) UnmarshalText(text []byte) error {
	switch string(text) {
	case "L1_GAS":
		*r = ResourceL1Gas
	case "L1_DATA_GAS":
		*r = ResourceL1DataGas
	case "L2_GAS":
		*r = ResourceL2Gas
	default:
		return fmt.Errorf("unknown resource: %q", string(text))
	}
	return nil
}

func (r Resource) MarshalText() ([]byte, error) {
	switch r {
	case ResourceL1Gas:
		return []byte("L1_GAS"), nil
	case ResourceL1DataGas:
		return []byte("L1_DATA_GAS"), nil
	case ResourceL2Gas:
		return []byte("L2_GAS"), nil
	default:
		return nil, errors.New("unknown resource")
	}
}

type DataAvailabilityMode uint32

const (
	DAModeL1 DataAvailabilityMode = iota
	DAModeL2
)

type ResourceBounds struct {
	MaxAmount       *felt.Felt `json:"max_amount"`
	MaxPricePerUnit *felt.Felt `json:"max_price_per_unit"`
}

// Transaction object returned by the feeder in JSON format for multiple endpoints
type Transaction struct {
	Hash                  *felt.Felt                   `json:"transaction_hash,omitempty"`
	Version               *felt.Felt                   `json:"version,omitempty"`
	ContractAddress       *felt.Felt                   `json:"contract_address,omitempty"`
	ContractAddressSalt   *felt.Felt                   `json:"contract_address_salt,omitempty"`
	ClassHash             *felt.Felt                   `json:"class_hash,omitempty"`
	ConstructorCallData   *[]*felt.Felt                `json:"constructor_calldata,omitempty"`
	Type                  TransactionType              `json:"type,omitempty"`
	SenderAddress         *felt.Felt                   `json:"sender_address,omitempty"`
	MaxFee                *felt.Felt                   `json:"max_fee,omitempty"`
	Signature             *[]*felt.Felt                `json:"signature,omitempty"`
	CallData              *[]*felt.Felt                `json:"calldata,omitempty"`
	EntryPointSelector    *felt.Felt                   `json:"entry_point_selector,omitempty"`
	Nonce                 *felt.Felt                   `json:"nonce,omitempty"`
	CompiledClassHash     *felt.Felt                   `json:"compiled_class_hash,omitempty"`
	ResourceBounds        *map[Resource]ResourceBounds `json:"resource_bounds,omitempty"`
	Tip                   *felt.Felt                   `json:"tip,omitempty"`
	NonceDAMode           *DataAvailabilityMode        `json:"nonce_data_availability_mode,omitempty"`
	FeeDAMode             *DataAvailabilityMode        `json:"fee_data_availability_mode,omitempty"`
	AccountDeploymentData *[]*felt.Felt                `json:"account_deployment_data,omitempty"`
	PaymasterData         *[]*felt.Felt                `json:"paymaster_data,omitempty"`
	ProofFacts            *[]*felt.Felt                `json:"proof_facts,omitempty"`
}

type TransactionFailureReason struct {
	Code    string `json:"code"`
	Message string `json:"error_message"`
}

type TransactionStatus struct {
	Status           string                    `json:"status"`
	FinalityStatus   FinalityStatus            `json:"finality_status"`
	ExecutionStatus  ExecutionStatus           `json:"execution_status"`
	BlockHash        *felt.Felt                `json:"block_hash"`
	BlockNumber      uint64                    `json:"block_number"`
	TransactionIndex uint64                    `json:"transaction_index"`
	Transaction      *Transaction              `json:"transaction"`
	RevertError      string                    `json:"revert_error"`
	FailureReason    *TransactionFailureReason `json:"transaction_failure_reason,omitempty"`
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
	DataAvailability       *DataAvailability      `json:"data_availability"`
	TotalGasConsumed       *GasConsumed           `json:"total_gas_consumed"`
}

type DataAvailability struct {
	L1Gas     uint64 `json:"l1_gas"`
	L1DataGas uint64 `json:"l1_data_gas"`
}

type BuiltinInstanceCounter struct {
	Pedersen     uint64 `json:"pedersen_builtin"`
	RangeCheck   uint64 `json:"range_check_builtin"`
	Bitwise      uint64 `json:"bitwise_builtin"`
	Output       uint64 `json:"output_builtin"`
	Ecsda        uint64 `json:"ecdsa_builtin"`
	EcOp         uint64 `json:"ec_op_builtin"`
	Keccak       uint64 `json:"keccak_builtin"`
	Poseidon     uint64 `json:"poseidon_builtin"`
	SegmentArena uint64 `json:"segment_arena_builtin"`
	AddMod       uint64 `json:"add_mod_builtin"`
	MulMod       uint64 `json:"mul_mod_builtin"`
	RangeCheck96 uint64 `json:"range_check96_builtin"`
}

type GasConsumed struct {
	L1Gas     uint64 `json:"l1_gas"`
	L1DataGas uint64 `json:"l1_data_gas"`
	L2Gas     uint64 `json:"l2_gas"`
}

type TransactionReceipt struct {
	// NOTE: finality_status is included on receipts retrieved from the get_transaction_receipt
	// endpoint, but is not included when receipt is in a block. We do not include the field
	// with an omitempty tag since it could cause very confusing behaviour. If the finality
	// status is needed, use get_block.

	ActualFee          *felt.Felt          `json:"actual_fee"`
	Events             []*Event            `json:"events"`
	ExecutionStatus    ExecutionStatus     `json:"execution_status"`
	ExecutionResources *ExecutionResources `json:"execution_resources"`
	L1ToL2Message      *L1ToL2Message      `json:"l1_to_l2_consumed_message"`
	L2ToL1Message      []*L2ToL1Message    `json:"l2_to_l1_messages"`
	TransactionHash    *felt.Felt          `json:"transaction_hash"`
	TransactionIndex   uint64              `json:"transaction_index"`
	RevertError        string              `json:"revert_error"`
}
