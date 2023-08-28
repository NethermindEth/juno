package utils

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
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

type ExecutionStatus uint8

const (
	Succeeded ExecutionStatus = iota + 1
	Reverted
)

func (es *ExecutionStatus) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case `"SUCCEEDED"`:
		*es = Succeeded
	case `"REVERTED"`:
		*es = Reverted
	default:
		return errors.New("unknown ExecutionStatus")
	}
	return nil
}

type FinalityStatus uint8

const (
	AcceptedOnL2 FinalityStatus = iota + 1
	AcceptedOnL1
	NotReceived
)

func (fs *FinalityStatus) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case `"ACCEPTED_ON_L2"`:
		*fs = AcceptedOnL2
	case `"ACCEPTED_ON_L1"`:
		*fs = AcceptedOnL1
	case `"NOT_RECEIVED"`:
		*fs = NotReceived
	default:
		return errors.New("unknown FinalityStatus")
	}
	return nil
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1252
//
//nolint:lll
type Transaction struct {
	Hash                *felt.Felt      `json:"transaction_hash,omitempty"`
	Type                TransactionType `json:"type" validate:"required"`
	Version             *felt.Felt      `json:"version,omitempty" validate:"required"`
	Nonce               *felt.Felt      `json:"nonce,omitempty" validate:"required_unless=Version 0x0"`
	MaxFee              *felt.Felt      `json:"max_fee,omitempty" validate:"required"`
	ContractAddress     *felt.Felt      `json:"contract_address,omitempty"`
	ContractAddressSalt *felt.Felt      `json:"contract_address_salt,omitempty" validate:"required_if=Type DEPLOY,required_if=Type DEPLOY_ACCOUNT"`
	ClassHash           *felt.Felt      `json:"class_hash,omitempty" validate:"required_if=Type DEPLOY,required_if=Type DEPLOY_ACCOUNT"`
	ConstructorCallData *[]*felt.Felt   `json:"constructor_calldata,omitempty" validate:"required_if=Type DEPLOY,required_if=Type DEPLOY_ACCOUNT"`
	SenderAddress       *felt.Felt      `json:"sender_address,omitempty" validate:"required_if=Type DECLARE,required_if=Type INVOKE Version 0x1"`
	Signature           *[]*felt.Felt   `json:"signature,omitempty" validate:"required"`
	CallData            *[]*felt.Felt   `json:"calldata,omitempty" validate:"required_if=Type INVOKE"`
	EntryPointSelector  *felt.Felt      `json:"entry_point_selector,omitempty" validate:"required_if=Type INVOKE Version 0x0"`
	CompiledClassHash   *felt.Felt      `json:"compiled_class_hash,omitempty" validate:"required_if=Type DECLARE Version 0x2"`
}

type TransactionStatus struct {
	Finality         FinalityStatus  `json:"finality_status"`
	Execution        ExecutionStatus `json:"execution_status"`
	Status           string          `json:"status"`
	BlockHash        *felt.Felt      `json:"block_hash"`
	BlockNumber      uint64          `json:"block_number"`
	TransactionIndex uint64          `json:"transaction_index"`
	Transaction      *Transaction    `json:"transaction"`
	RevertError      string          `json:"revert_error"`
}

type MsgFromL1 struct {
	// The address of the L1 contract sending the message.
	From common.Address `json:"from_address" validate:"required"`
	// The address of the L1 contract sending the message.
	To felt.Felt `json:"to_address" validate:"required"`
	// The payload of the message.
	Payload  []felt.Felt `json:"payload" validate:"required"`
	Selector felt.Felt   `json:"entry_point_selector" validate:"required"`
	Nonce    *felt.Felt  `json:"nonce"`
}

type MsgToL1 struct {
	From    *felt.Felt     `json:"from_address"`
	To      common.Address `json:"to_address"`
	Payload []*felt.Felt   `json:"payload"`
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

type Event struct {
	From *felt.Felt   `json:"from_address"`
	Keys []*felt.Felt `json:"keys"`
	Data []*felt.Felt `json:"data"`
}

// https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_api_openrpc.json#L1871
type TransactionReceipt struct {
	Type            TransactionType `json:"type"`
	Hash            *felt.Felt      `json:"transaction_hash"`
	ActualFee       *felt.Felt      `json:"actual_fee"`
	ExecutionStatus ExecutionStatus `json:"execution_status"`
	FinalityStatus  FinalityStatus  `json:"finality_status"`
	BlockHash       *felt.Felt      `json:"block_hash,omitempty"`
	BlockNumber     *uint64         `json:"block_number,omitempty"`
	MessagesSent    []*MsgToL1      `json:"messages_sent"`
	Events          []*Event        `json:"events"`
	ContractAddress *felt.Felt      `json:"contract_address,omitempty"`
	RevertReason    string          `json:"revert_reason,omitempty"`
}

type TransactionReceiptSeq struct {
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
	network core.Network,
) (core.Transaction, core.Class, *felt.Felt, error) {
	var feederTxn Transaction
	if err := copier.Copy(&feederTxn, broadcastedTxn.Transaction); err != nil {
		return nil, nil, nil, err
	}

	txn, err := AdaptTransaction(&feederTxn)
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
			t.ClassHash, err = Cairo0ClassHash(c)
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
