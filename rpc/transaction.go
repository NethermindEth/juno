package rpc

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/adapters/feeder2core"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
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
	case "\"DECLARE\"":
		*t = TxnDeclare
	case "\"DEPLOY\"":
		*t = TxnDeploy
	case "\"DEPLOY_ACCOUNT\"":
		*t = TxnDeployAccount
	case "\"INVOKE\"", "\"INVOKE_FUNCTION\"":
		*t = TxnInvoke
	case "\"L1_HANDLER\"":
		*t = TxnL1Handler
	default:
		return errors.New("unknown TransactionType")
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

type MsgToL1 struct {
	From    *felt.Felt     `json:"from_address"`
	To      common.Address `json:"to_address"`
	Payload []*felt.Felt   `json:"payload"`
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
	Status          Status          `json:"status"`
	BlockHash       *felt.Felt      `json:"block_hash,omitempty"`
	BlockNumber     *uint64         `json:"block_number,omitempty"`
	MessagesSent    []*MsgToL1      `json:"messages_sent"`
	Events          []*Event        `json:"events"`
	ContractAddress *felt.Felt      `json:"contract_address,omitempty"`
}

type AddInvokeTxResponse struct {
	TransactionHash *felt.Felt `json:"transaction_hash"`
}

type DeployAccountTxResponse struct {
	TransactionHash *felt.Felt `json:"transaction_hash"`
	ContractAddress *felt.Felt `json:"contract_address"`
}

type DeclareTxResponse struct {
	TransactionHash *felt.Felt `json:"transaction_hash"`
	ClassHash       *felt.Felt `json:"class_hash"`
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1273-L1287
type BroadcastedTransaction struct {
	Transaction
	ContractClass json.RawMessage `json:"contract_class,omitempty" validate:"required_if=Transaction.Type DECLARE"`
}

type FeeEstimate struct {
	GasConsumed *felt.Felt `json:"gas_consumed"`
	GasPrice    *felt.Felt `json:"gas_price"`
	OverallFee  *felt.Felt `json:"overall_fee"`
}

func adaptBroadcastedTransaction(broadcastedTxn *BroadcastedTransaction, network utils.Network) (core.Transaction, core.Class, error) {
	var feederTxn feeder.Transaction
	if err := copier.Copy(&feederTxn, broadcastedTxn.Transaction); err != nil {
		return nil, nil, err
	}
	feederTxn.Type = broadcastedTxn.Type.String()

	txn, err := feeder2core.AdaptTransaction(&feederTxn)
	if err != nil {
		return nil, nil, err
	}

	var declaredClass core.Class
	if len(broadcastedTxn.ContractClass) != 0 {
		declaredClass, err = adaptDeclaredClass(broadcastedTxn.ContractClass)
		if err != nil {
			return nil, nil, err
		}
	} else if broadcastedTxn.Type == TxnDeclare {
		return nil, nil, errors.New("declare without a class definition")
	}

	if t, ok := txn.(*core.DeclareTransaction); ok {
		switch c := declaredClass.(type) {
		case *core.Cairo0Class:
			t.ClassHash, err = vm.Cairo0ClassHash(c)
			if err != nil {
				return nil, nil, err
			}
		case *core.Cairo1Class:
			t.ClassHash = c.Hash()
		}
	}

	txnHash, err := core.TransactionHash(txn, network)
	if err != nil {
		return nil, nil, err
	}

	switch t := txn.(type) {
	case *core.DeclareTransaction:
		t.TransactionHash = txnHash
	case *core.InvokeTransaction:
		t.TransactionHash = txnHash
	case *core.DeployAccountTransaction:
		t.TransactionHash = txnHash
	default:
		return nil, nil, errors.New("unsupported transaction")
	}

	if txn.Hash() == nil {
		return nil, nil, errors.New("deprecated transaction type")
	}
	return txn, declaredClass, nil
}
