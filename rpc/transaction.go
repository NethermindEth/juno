package rpc

import (
	"errors"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/ethereum/go-ethereum/common"
)

type TransactionType uint8

const (
	TxnDeclare TransactionType = iota
	TxnDeploy
	TxnDeployAccount
	TxnInvoke
	TxnL1Handler
)

func (t TransactionType) MarshalJSON() ([]byte, error) {
	switch t {
	case TxnDeclare:
		return []byte("\"DECLARE\""), nil
	case TxnDeploy:
		return []byte("\"DEPLOY\""), nil
	case TxnDeployAccount:
		return []byte("\"DEPLOY_ACCOUNT\""), nil
	case TxnInvoke:
		return []byte("\"INVOKE\""), nil
	case TxnL1Handler:
		return []byte("\"L1_HANDLER\""), nil
	default:
		return nil, errors.New("unknown TransactionType")
	}
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1252
type Transaction struct {
	Hash                *felt.Felt      `json:"transaction_hash,omitempty"`
	Type                TransactionType `json:"type"`
	Version             *felt.Felt      `json:"version,omitempty"`
	Nonce               *felt.Felt      `json:"nonce,omitempty"`
	MaxFee              *felt.Felt      `json:"max_fee,omitempty"`
	ContractAddress     *felt.Felt      `json:"contract_address,omitempty"`
	ContractAddressSalt *felt.Felt      `json:"contract_address_salt,omitempty"`
	ClassHash           *felt.Felt      `json:"class_hash,omitempty"`
	ConstructorCalldata []*felt.Felt    `json:"constructor_calldata,omitempty"`
	SenderAddress       *felt.Felt      `json:"sender_address,omitempty"`
	Signature           *[]*felt.Felt   `json:"signature,omitempty"`
	Calldata            *[]*felt.Felt   `json:"calldata,omitempty"`
	EntryPointSelector  *felt.Felt      `json:"entry_point_selector,omitempty"`
	CompiledClassHash   *felt.Felt      `json:"compiled_class_hash,omitempty"`
}

type MsgToL1 struct {
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
	BlockHash       *felt.Felt      `json:"block_hash"`
	BlockNumber     uint64          `json:"block_number"`
	MessagesSent    []*MsgToL1      `json:"messages_sent"`
	Events          []*Event        `json:"events"`
	ContractAddress *felt.Felt      `json:"contract_address,omitempty"`
}
