package rpc

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/jinzhu/copier"
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

func (t TransactionType) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", t.String())), nil
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

type FeeUnit byte

const (
	WEI FeeUnit = iota
	STRK
)

func (u FeeUnit) MarshalJSON() ([]byte, error) {
	switch u {
	case WEI:
		return []byte(`"WEI"`), nil
	case STRK:
		return []byte(`"STRK"`), nil
	default:
		return nil, errors.New("unknown FeeUnit")
	}
}

type TxnStatus uint8

const (
	TxnStatusAcceptedOnL1 TxnStatus = iota + 1
	TxnStatusAcceptedOnL2
	TxnStatusReceived
	TxnStatusRejected
)

func (s TxnStatus) MarshalJSON() ([]byte, error) {
	switch s {
	case TxnStatusReceived:
		return []byte(`"RECEIVED"`), nil
	case TxnStatusRejected:
		return []byte(`"REJECTED"`), nil
	case TxnStatusAcceptedOnL1:
		return []byte(`"ACCEPTED_ON_L1"`), nil
	case TxnStatusAcceptedOnL2:
		return []byte(`"ACCEPTED_ON_L2"`), nil
	default:
		return nil, errors.New("unknown ExecutionStatus")
	}
}

type TxnExecutionStatus uint8

const (
	TxnSuccess TxnExecutionStatus = iota + 1
	TxnFailure
)

func (es TxnExecutionStatus) MarshalJSON() ([]byte, error) {
	switch es {
	case TxnSuccess:
		return []byte(`"SUCCEEDED"`), nil
	case TxnFailure:
		return []byte(`"REVERTED"`), nil
	default:
		return nil, errors.New("unknown ExecutionStatus")
	}
}

type TxnFinalityStatus uint8

const (
	TxnAcceptedOnL1 TxnFinalityStatus = iota + 1
	TxnAcceptedOnL2
)

func (fs TxnFinalityStatus) MarshalJSON() ([]byte, error) {
	switch fs {
	case TxnAcceptedOnL1:
		return []byte(`"ACCEPTED_ON_L1"`), nil
	case TxnAcceptedOnL2:
		return []byte(`"ACCEPTED_ON_L2"`), nil
	default:
		return nil, errors.New("unknown FinalityStatus")
	}
}

type DataAvailabilityMode uint32

const (
	DAModeL1 DataAvailabilityMode = iota
	DAModeL2
)

func (m DataAvailabilityMode) MarshalJSON() ([]byte, error) {
	switch m {
	case DAModeL1:
		return []byte(`"L1"`), nil
	case DAModeL2:
		return []byte(`"L2"`), nil
	default:
		return nil, errors.New("unknown DataAvailabilityMode")
	}
}

type Resource uint32

const (
	ResourceL1Gas Resource = iota + 1
	ResourceL2Gas
)

func (r Resource) MarshalJSON() ([]byte, error) {
	switch r {
	case ResourceL1Gas:
		return []byte("l1_gas"), nil
	case ResourceL2Gas:
		return []byte("l2_gas"), nil
	default:
		return nil, errors.New("unknown Resource")
	}
}

func (r Resource) MarshalText() ([]byte, error) {
	return r.MarshalJSON()
}

type ResourceBounds struct {
	MaxAmount       *felt.Felt `json:"max_amount"`
	MaxPricePerUnit *felt.Felt `json:"max_price_per_unit"`
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1252
//
//nolint:lll
type Transaction struct {
	Hash                  *felt.Felt                   `json:"transaction_hash,omitempty"`
	Type                  TransactionType              `json:"type" validate:"required"`
	Version               *felt.Felt                   `json:"version,omitempty" validate:"required"`
	Nonce                 *felt.Felt                   `json:"nonce,omitempty" validate:"required_unless=Version 0x0"`
	MaxFee                *felt.Felt                   `json:"max_fee,omitempty" validate:"required_if=Version 0x0,required_if=Version 0x1,required_if=Version 0x2"`
	ContractAddress       *felt.Felt                   `json:"contract_address,omitempty"`
	ContractAddressSalt   *felt.Felt                   `json:"contract_address_salt,omitempty" validate:"required_if=Type DEPLOY,required_if=Type DEPLOY_ACCOUNT"`
	ClassHash             *felt.Felt                   `json:"class_hash,omitempty" validate:"required_if=Type DEPLOY,required_if=Type DEPLOY_ACCOUNT"`
	ConstructorCallData   *[]*felt.Felt                `json:"constructor_calldata,omitempty" validate:"required_if=Type DEPLOY,required_if=Type DEPLOY_ACCOUNT"`
	SenderAddress         *felt.Felt                   `json:"sender_address,omitempty" validate:"required_if=Type DECLARE,required_if=Type INVOKE Version 0x1"`
	Signature             *[]*felt.Felt                `json:"signature,omitempty" validate:"required"`
	CallData              *[]*felt.Felt                `json:"calldata,omitempty" validate:"required_if=Type INVOKE"`
	EntryPointSelector    *felt.Felt                   `json:"entry_point_selector,omitempty" validate:"required_if=Type INVOKE Version 0x0"`
	CompiledClassHash     *felt.Felt                   `json:"compiled_class_hash,omitempty" validate:"required_if=Type DECLARE Version 0x2"`
	ResourceBounds        *map[Resource]ResourceBounds `json:"resource_bounds,omitempty" validate:"required_if=Version 0x3"`
	Tip                   *felt.Felt                   `json:"tip,omitempty" validate:"required_if=Version 0x3"`
	PaymasterData         *[]*felt.Felt                `json:"paymaster_data,omitempty" validate:"required_if=Version 0x3"`
	AccountDeploymentData *[]*felt.Felt                `json:"account_deployment_data,omitempty" validate:"required_if=Type INVOKE Version 0x3,required_if=Type DECLARE Version 0x3"`
	NonceDAMode           *DataAvailabilityMode        `json:"nonce_data_availability_mode,omitempty" validate:"required_if=Version 0x3"`
	FeeDAMode             *DataAvailabilityMode        `json:"fee_data_availability_mode,omitempty" validate:"required_if=Version 0x3"`
}

var (
	felt3 = new(felt.Felt).SetUint64(3)
	felt1 = new(felt.Felt).SetUint64(1)
	felt2 = new(felt.Felt).SetUint64(2)
)

func (tx *Transaction) ToPreV3() error {
	if !tx.Version.Equal(felt3) {
		return nil
	}
	switch tx.Type {
	case TxnDeclare:
		tx.Version.Set(felt2)
	case TxnInvoke, TxnDeployAccount:
		tx.Version.Set(felt1)
	default:
		return fmt.Errorf("unexpected transaction type %s", tx.Type)
	}
	l1Resources := (*tx.ResourceBounds)[ResourceL1Gas]
	tx.MaxFee = new(felt.Felt).Mul(l1Resources.MaxAmount, l1Resources.MaxPricePerUnit)

	tx.ResourceBounds = nil
	tx.Tip = nil
	tx.PaymasterData = nil
	tx.AccountDeploymentData = nil
	tx.NonceDAMode = nil
	tx.FeeDAMode = nil
	return nil
}

type TransactionStatus struct {
	Finality  TxnStatus          `json:"finality_status"`
	Execution TxnExecutionStatus `json:"execution_status,omitempty"`
}

type MsgFromL1 struct {
	// The address of the L1 contract sending the message.
	From common.Address `json:"from_address" validate:"required"`
	// The address of the L1 contract sending the message.
	To felt.Felt `json:"to_address" validate:"required"`
	// The payload of the message.
	Payload  []felt.Felt `json:"payload" validate:"required"`
	Selector felt.Felt   `json:"entry_point_selector" validate:"required"`
}

type MsgToL1 struct {
	From    *felt.Felt     `json:"from_address,omitempty"`
	To      common.Address `json:"to_address"`
	Payload []*felt.Felt   `json:"payload"`
}

type Event struct {
	From *felt.Felt   `json:"from_address,omitempty"`
	Keys []*felt.Felt `json:"keys"`
	Data []*felt.Felt `json:"data"`
}

type ExecutionResources struct {
	Steps        uint64  `json:"steps"`
	MemoryHoles  *uint64 `json:"memory_holes,omitempty"`
	Pedersen     *uint64 `json:"pedersen_builtin_applications,omitempty"`
	RangeCheck   *uint64 `json:"range_check_builtin_applications,omitempty"`
	Bitwise      *uint64 `json:"bitwise_builtin_applications,omitempty"`
	Ecsda        *uint64 `json:"ecdsa_builtin_applications,omitempty"`
	EcOp         *uint64 `json:"ec_op_builtin_applications,omitempty"`
	Keccak       *uint64 `json:"keccak_builtin_applications,omitempty"`
	Poseidon     *uint64 `json:"poseidon_builtin_applications,omitempty"`
	SegmentArena *uint64 `json:"segment_arena_builtin,omitempty"`
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

type FeePayment struct {
	Amount   *felt.Felt `json:"amount"`
	Unit     FeeUnit    `json:"unit"`
	isLegacy bool
}

func (f *FeePayment) MarshalJSON() ([]byte, error) {
	if f.isLegacy {
		return json.Marshal(f.Amount)
	}
	type fee FeePayment // Avoid infinite recursion with MarshalJSON.
	return json.Marshal(fee(*f))
}

type AddTxResponse struct {
	TransactionHash *felt.Felt `json:"transaction_hash"`
	ContractAddress *felt.Felt `json:"contract_address,omitempty"`
	ClassHash       *felt.Felt `json:"class_hash,omitempty"`
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1273-L1287
type BroadcastedTransaction struct {
	Transaction
	ContractClass json.RawMessage `json:"contract_class,omitempty" validate:"required_if=Transaction.Type DECLARE"`
	PaidFeeOnL1   *felt.Felt      `json:"paid_fee_on_l1,omitempty" validate:"required_if=Transaction.Type L1_HANDLER"`
}

type FeeEstimate struct {
	GasConsumed *felt.Felt `json:"gas_consumed"`
	GasPrice    *felt.Felt `json:"gas_price"`
	OverallFee  *felt.Felt `json:"overall_fee"`
}

//nolint:gocyclo
func adaptBroadcastedTransaction(broadcastedTxn *BroadcastedTransaction,
	network utils.Network,
) (core.Transaction, core.Class, *felt.Felt, error) {
	var feederTxn starknet.Transaction
	if err := copier.Copy(&feederTxn, broadcastedTxn.Transaction); err != nil {
		return nil, nil, nil, err
	}

	txn, err := sn2core.AdaptTransaction(&feederTxn)
	if err != nil {
		return nil, nil, nil, err
	}

	var declaredClass core.Class
	if len(broadcastedTxn.ContractClass) != 0 {
		declaredClass, err = adaptDeclaredClass(broadcastedTxn.ContractClass)
		if err != nil {
			return nil, nil, nil, err
		}
	} else if broadcastedTxn.Type == TxnDeclare {
		return nil, nil, nil, errors.New("declare without a class definition")
	}

	if t, ok := txn.(*core.DeclareTransaction); ok {
		switch c := declaredClass.(type) {
		case *core.Cairo0Class:
			t.ClassHash, err = vm.Cairo0ClassHash(c)
			if err != nil {
				return nil, nil, nil, err
			}
		case *core.Cairo1Class:
			t.ClassHash = c.Hash()
		}
	}

	txnHash, err := core.TransactionHash(txn, network)
	if err != nil {
		return nil, nil, nil, err
	}

	var paidFeeOnL1 *felt.Felt
	switch t := txn.(type) {
	case *core.DeclareTransaction:
		t.TransactionHash = txnHash
	case *core.InvokeTransaction:
		t.TransactionHash = txnHash
	case *core.DeployAccountTransaction:
		t.TransactionHash = txnHash
	case *core.L1HandlerTransaction:
		t.TransactionHash = txnHash
		paidFeeOnL1 = broadcastedTxn.PaidFeeOnL1
	default:
		return nil, nil, nil, errors.New("unsupported transaction")
	}

	if txn.Hash() == nil {
		return nil, nil, nil, errors.New("deprecated transaction type")
	}
	return txn, declaredClass, paidFeeOnL1, nil
}

func adaptResourceBounds(rb map[core.Resource]core.ResourceBounds) map[Resource]ResourceBounds {
	rpcResourceBounds := make(map[Resource]ResourceBounds)
	for resource, bounds := range rb {
		rpcResourceBounds[Resource(resource)] = ResourceBounds{
			MaxAmount:       new(felt.Felt).SetUint64(bounds.MaxAmount),
			MaxPricePerUnit: bounds.MaxPricePerUnit,
		}
	}
	return rpcResourceBounds
}
