package rpcv10

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/ethereum/go-ethereum/common"
)

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

type TxnStatus uint8

const (
	TxnStatusReceived TxnStatus = iota + 1
	TxnStatusCandidate
	TxnStatusPreConfirmed
	TxnStatusAcceptedOnL2
	TxnStatusAcceptedOnL1
)

func (s TxnStatus) MarshalText() ([]byte, error) {
	switch s {
	case TxnStatusReceived:
		return []byte("RECEIVED"), nil
	case TxnStatusAcceptedOnL1:
		return []byte("ACCEPTED_ON_L1"), nil
	case TxnStatusAcceptedOnL2:
		return []byte("ACCEPTED_ON_L2"), nil
	case TxnStatusCandidate:
		return []byte("CANDIDATE"), nil
	case TxnStatusPreConfirmed:
		return []byte("PRE_CONFIRMED"), nil
	default:
		return nil, fmt.Errorf("unknown TxnStatus %v", s)
	}
}

type TxnExecutionStatus uint8

const (
	UnknownExecution TxnExecutionStatus = iota
	TxnSuccess
	TxnFailure
)

func (es TxnExecutionStatus) MarshalText() ([]byte, error) {
	switch es {
	case TxnSuccess:
		return []byte("SUCCEEDED"), nil
	case TxnFailure:
		return []byte("REVERTED"), nil
	default:
		return nil, fmt.Errorf("unknown ExecutionStatus %v", es)
	}
}

func (es *TxnExecutionStatus) UnmarshalText(text []byte) error {
	switch string(text) {
	case "SUCCEEDED":
		*es = TxnSuccess
	case "REVERTED":
		*es = TxnFailure
	default:
		return fmt.Errorf("unknown ExecutionStatus %s", string(text))
	}
	return nil
}

// TxnFinalityStatus represents the finality status of a transaction.
type TxnFinalityStatus uint8

const (
	// Starts from 3 for TxnFinalityStatuses to match same numbers on TxnStatus
	TxnPreConfirmed TxnFinalityStatus = iota + 3
	TxnAcceptedOnL2
	TxnAcceptedOnL1
)

func (fs TxnFinalityStatus) MarshalText() ([]byte, error) {
	switch fs {
	case TxnPreConfirmed:
		return []byte("PRE_CONFIRMED"), nil
	case TxnAcceptedOnL1:
		return []byte("ACCEPTED_ON_L1"), nil
	case TxnAcceptedOnL2:
		return []byte("ACCEPTED_ON_L2"), nil
	default:
		return nil, fmt.Errorf("unknown FinalityStatus %v", fs)
	}
}

func (fs *TxnFinalityStatus) UnmarshalText(text []byte) error {
	switch string(text) {
	case "PRE_CONFIRMED":
		*fs = TxnPreConfirmed
	case "ACCEPTED_ON_L1":
		*fs = TxnAcceptedOnL1
	case "ACCEPTED_ON_L2":
		*fs = TxnAcceptedOnL2
	default:
		return fmt.Errorf("unknown FinalityStatus %s", string(text))
	}
	return nil
}

type DataAvailabilityMode uint32

const (
	DAModeL1 DataAvailabilityMode = iota
	DAModeL2
)

func (m DataAvailabilityMode) MarshalText() ([]byte, error) {
	switch m {
	case DAModeL1:
		return []byte("L1"), nil
	case DAModeL2:
		return []byte("L2"), nil
	default:
		return nil, fmt.Errorf("unknown DataAvailabilityMode %v", m)
	}
}

func (m *DataAvailabilityMode) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case `"L1"`:
		*m = DAModeL1
	case `"L2"`:
		*m = DAModeL2
	default:
		return fmt.Errorf("unknown DataAvailabilityMode: %q", string(data))
	}
	return nil
}

type Resource uint32

const (
	ResourceL1Gas Resource = iota + 1
	ResourceL2Gas
	ResourceL1DataGas
)

func (r Resource) MarshalText() ([]byte, error) {
	switch r {
	case ResourceL1Gas:
		return []byte("l1_gas"), nil
	case ResourceL2Gas:
		return []byte("l2_gas"), nil
	case ResourceL1DataGas:
		return []byte("l1_data_gas"), nil
	default:
		return nil, fmt.Errorf("unknown Resource %v", r)
	}
}

func (r *Resource) UnmarshalJSON(data []byte) error {
	str := string(data)
	switch strings.ToLower(strings.Trim(str, `"`)) {
	case "l1_gas":
		*r = ResourceL1Gas
	case "l2_gas":
		*r = ResourceL2Gas
	case "l1_data_gas":
		*r = ResourceL1DataGas
	default:
		return fmt.Errorf("unknown Resource: %q", str)
	}
	return nil
}

func (r *Resource) UnmarshalText(data []byte) error {
	return r.UnmarshalJSON(data)
}

type ResourceBounds struct {
	MaxAmount       *felt.Felt `json:"max_amount"`
	MaxPricePerUnit *felt.Felt `json:"max_price_per_unit"`
}

// TODO: using Value fields here is a good idea, however
// we are currently keeping the field's type Reference since the current
// validation tags we are using does not work well with Value field.
// We should revisit this when we start implementing custom validations.
type ResourceBoundsMap struct {
	L1Gas     *ResourceBounds `json:"l1_gas" validate:"required"`
	L2Gas     *ResourceBounds `json:"l2_gas" validate:"required"`
	L1DataGas *ResourceBounds `json:"l1_data_gas" validate:"required"`
}

func (r *ResourceBoundsMap) MarshalJSON() ([]byte, error) {
	// Check if L1DataGas is nil, if it is, provide default values
	if r.L1DataGas == nil {
		r.L1DataGas = &ResourceBounds{
			MaxAmount:       &felt.Zero,
			MaxPricePerUnit: &felt.Zero,
		}
	}

	// Define an alias to avoid recursion
	type alias ResourceBoundsMap
	return json.Marshal((*alias)(r))
}

type FeePayment struct {
	Amount *felt.Felt `json:"amount"`
	Unit   FeeUnit    `json:"unit"`
}

type MsgToL1 struct {
	From    *felt.Felt     `json:"from_address,omitempty"`
	To      common.Address `json:"to_address"`
	Payload []*felt.Felt   `json:"payload"`
}

type ComputationResources struct {
	Steps        uint64 `json:"steps"`
	MemoryHoles  uint64 `json:"memory_holes,omitempty"`
	Pedersen     uint64 `json:"pedersen_builtin_applications,omitempty"`
	RangeCheck   uint64 `json:"range_check_builtin_applications,omitempty"`
	Bitwise      uint64 `json:"bitwise_builtin_applications,omitempty"`
	Ecsda        uint64 `json:"ecdsa_builtin_applications,omitempty"`
	EcOp         uint64 `json:"ec_op_builtin_applications,omitempty"`
	Keccak       uint64 `json:"keccak_builtin_applications,omitempty"`
	Poseidon     uint64 `json:"poseidon_builtin_applications,omitempty"`
	SegmentArena uint64 `json:"segment_arena_builtin,omitempty"`
}

type InnerExecutionResources struct {
	L1Gas uint64 `json:"l1_gas"`
	L2Gas uint64 `json:"l2_gas"`
}

type ExecutionResources struct {
	InnerExecutionResources
	L1DataGas uint64 `json:"l1_data_gas"`
}

// https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_api_openrpc.json#L1871
type TransactionReceipt struct {
	Type               TransactionType     `json:"type"`
	Hash               *felt.Felt          `json:"transaction_hash"`
	ActualFee          *FeePayment         `json:"actual_fee"`
	ExecutionStatus    TxnExecutionStatus  `json:"execution_status"`
	FinalityStatus     TxnFinalityStatus   `json:"finality_status"`
	BlockHash          *felt.Felt          `json:"block_hash,omitempty"`
	BlockNumber        *uint64             `json:"block_number,omitempty"`
	MessagesSent       []*MsgToL1          `json:"messages_sent"`
	Events             []*Event            `json:"events"`
	ContractAddress    *felt.Felt          `json:"contract_address,omitempty"`
	RevertReason       string              `json:"revert_reason,omitempty"`
	ExecutionResources *ExecutionResources `json:"execution_resources,omitempty"`
	MessageHash        string              `json:"message_hash,omitempty"`
}

type CalldataInputs = rpccore.LimitSlice[felt.Felt, rpccore.FunctionCalldataLimit]

// https://github.com/starkware-libs/starknet-specs/blob/v0.3.0/api/starknet_api_openrpc.json#L2344
type FunctionCall struct {
	ContractAddress    felt.Felt      `json:"contract_address"`
	EntryPointSelector felt.Felt      `json:"entry_point_selector"`
	Calldata           CalldataInputs `json:"calldata"`
}

type TransactionStatus struct {
	Finality      TxnStatus          `json:"finality_status"`
	Execution     TxnExecutionStatus `json:"execution_status,omitempty"`
	FailureReason string             `json:"failure_reason,omitempty"`
}

type FeeUnit byte

const (
	WEI FeeUnit = iota
	FRI
)

func (u FeeUnit) MarshalText() ([]byte, error) {
	switch u {
	case WEI:
		return []byte("WEI"), nil
	case FRI:
		return []byte("FRI"), nil
	default:
		return nil, fmt.Errorf("unknown FeeUnit %v", u)
	}
}

func (u *FeeUnit) UnmarshalText(text []byte) error {
	switch string(text) {
	case "WEI":
		*u = WEI
	case "FRI":
		*u = FRI
	default:
		return fmt.Errorf("unknown FeeUnit %s", string(text))
	}
	return nil
}

type FeeEstimate struct {
	L1GasConsumed     *felt.Felt `json:"l1_gas_consumed,omitempty"`
	L1GasPrice        *felt.Felt `json:"l1_gas_price,omitempty"`
	L2GasConsumed     *felt.Felt `json:"l2_gas_consumed,omitempty"`
	L2GasPrice        *felt.Felt `json:"l2_gas_price,omitempty"`
	L1DataGasConsumed *felt.Felt `json:"l1_data_gas_consumed,omitempty"`
	L1DataGasPrice    *felt.Felt `json:"l1_data_gas_price,omitempty"`
	OverallFee        *felt.Felt `json:"overall_fee"`
	Unit              *FeeUnit   `json:"unit,omitempty"`
}

type ContractErrorData struct {
	RevertError json.RawMessage `json:"revert_error"`
}

func MakeContractError(err json.RawMessage) *jsonrpc.Error {
	return rpccore.ErrContractError.CloneWithData(ContractErrorData{
		RevertError: err,
	})
}

func MakeTransactionExecutionError(err *vm.TransactionExecutionError) *jsonrpc.Error {
	return rpccore.ErrTransactionExecutionError.CloneWithData(TransactionExecutionErrorData{
		TransactionIndex: err.Index,
		ExecutionError:   err.Cause,
	})
}

func adaptExecutionResources(resources *core.ExecutionResources) *ExecutionResources {
	if resources == nil {
		return &ExecutionResources{}
	}

	res := &ExecutionResources{}
	if tgc := resources.TotalGasConsumed; tgc != nil {
		res.L1Gas = tgc.L1Gas
		res.L2Gas = tgc.L2Gas
		res.L1DataGas = tgc.L1DataGas
	}
	return res
}

// Transaction represents a Starknet transaction in the RPC API.
type Transaction struct {
	Hash                  *felt.Felt            `json:"transaction_hash,omitempty"`
	Type                  TransactionType       `json:"type" validate:"required"`
	Version               *felt.Felt            `json:"version,omitempty" validate:"required,version_0x3"` //nolint:lll,nolintlint // conflicting line limits
	Nonce                 *felt.Felt            `json:"nonce,omitempty" validate:"required"`
	MaxFee                *felt.Felt            `json:"max_fee,omitempty"`
	ContractAddress       *felt.Felt            `json:"contract_address,omitempty"`
	ContractAddressSalt   *felt.Felt            `json:"contract_address_salt,omitempty" validate:"required_if=Type DEPLOY,required_if=Type DEPLOY_ACCOUNT"` //nolint:lll // tag
	ClassHash             *felt.Felt            `json:"class_hash,omitempty" validate:"required_if=Type DEPLOY,required_if=Type DEPLOY_ACCOUNT"`            //nolint:lll // tag
	ConstructorCallData   *[]*felt.Felt         `json:"constructor_calldata,omitempty" validate:"required_if=Type DEPLOY,required_if=Type DEPLOY_ACCOUNT"`  //nolint:lll // tag
	SenderAddress         *felt.Felt            `json:"sender_address,omitempty" validate:"required_if=Type DECLARE,required_if=Type INVOKE"`               //nolint:lll // tag
	Signature             *[]*felt.Felt         `json:"signature,omitempty" validate:"required"`
	CallData              *[]*felt.Felt         `json:"calldata,omitempty" validate:"required_if=Type INVOKE"` //nolint:lll,nolintlint // conflicting line limits
	EntryPointSelector    *felt.Felt            `json:"entry_point_selector,omitempty"`
	CompiledClassHash     *felt.Felt            `json:"compiled_class_hash,omitempty"`
	ResourceBounds        *ResourceBoundsMap    `json:"resource_bounds,omitempty" validate:"resource_bounds_required"` //nolint:lll,nolintlint // conflicting line limits
	Tip                   *felt.Felt            `json:"tip,omitempty" validate:"required"`
	PaymasterData         *[]*felt.Felt         `json:"paymaster_data,omitempty" validate:"required"`
	AccountDeploymentData *[]*felt.Felt         `json:"account_deployment_data,omitempty" validate:"required_if=Type INVOKE,required_if=Type DECLARE"` //nolint:lll // tag
	NonceDAMode           *DataAvailabilityMode `json:"nonce_data_availability_mode,omitempty" validate:"required"`                                    //nolint:lll,nolintlint // conflicting line limits
	FeeDAMode             *DataAvailabilityMode `json:"fee_data_availability_mode,omitempty" validate:"required"`                                      //nolint:lll,nolintlint // conflicting line limits
	ProofFacts            []*felt.Felt          `json:"proof_facts,omitempty"`
}

// BroadcastedTransaction represents a transaction submitted via the RPC API.
type BroadcastedTransaction struct {
	Transaction
	ContractClass json.RawMessage `json:"contract_class,omitempty" validate:"required_if=Transaction.Type DECLARE"`    //nolint:lll // validate tag
	PaidFeeOnL1   *felt.Felt      `json:"paid_fee_on_l1,omitempty" validate:"required_if=Transaction.Type L1_HANDLER"` //nolint:lll // validate tag
	Proof         utils.Base64    `json:"proof,omitempty" validate:"excluded_unless=Type INVOKE,omitempty,base64"`     //nolint:lll // validate tag
	ProofFacts    []felt.Felt     `json:"proof_facts,omitempty" validate:"excluded_unless=Type INVOKE"`
}

type TransactionExecutionErrorData struct {
	TransactionIndex uint64          `json:"transaction_index"`
	ExecutionError   json.RawMessage `json:"execution_error"`
}

// todo(rdr): This should be modified to receive a transaction version instead
// and the if conditions can be simplified by just asking if version is 3 :)
func feeUnit(txn core.Transaction) FeeUnit {
	feeUnit := WEI
	version := txn.TxVersion()
	if !version.Is(0) && !version.Is(1) && !version.Is(2) {
		feeUnit = FRI
	}

	return feeUnit
}
