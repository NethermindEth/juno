package types

import (
	"github.com/NethermindEth/juno/pkg/felt"
)

type IsTransaction interface {
	GetHash() *felt.Felt
}

type TransactionDeploy struct {
	Hash                *felt.Felt
	ContractAddress     *felt.Felt
	ConstructorCallData []*felt.Felt
}

func (tx *TransactionDeploy) GetHash() *felt.Felt {
	return tx.Hash
}

type TransactionInvoke struct {
	Hash               *felt.Felt   `json:"txn_hash"`
	ContractAddress    *felt.Felt   `json:"contract_address"`
	EntryPointSelector *felt.Felt   `json:"entry_point_selector"`
	CallData           []*felt.Felt `json:"calldata"`
	Signature          []*felt.Felt `json:"-"`
	MaxFee             *felt.Felt   `json:"max_fee"`
}

func (tx *TransactionInvoke) GetHash() *felt.Felt {
	return tx.Hash
}

type TransactionDeclare struct {
	Hash          *felt.Felt
	ClassHash     *felt.Felt
	SenderAddress *felt.Felt
	MaxFee        *felt.Felt
	Signature     []*felt.Felt
	Nonce         *felt.Felt
	Version       *felt.Felt
}

func (tx *TransactionDeclare) GetHash() *felt.Felt {
	return tx.Hash
}

type TransactionStatus int64

const (
	TxStatusUnknown TransactionStatus = iota
	TxStatusNotReceived
	TxStatusReceived
	TxStatusPending
	TxStatusRejected
	TxStatusAcceptedOnL2
	TxStatusAcceptedOnL1
)

var (
	TxStatusName = map[TransactionStatus]string{
		TxStatusUnknown:      "UNKNOWN",
		TxStatusNotReceived:  "NOT_RECEIVED",
		TxStatusReceived:     "RECEIVED",
		TxStatusPending:      "PENDING",
		TxStatusRejected:     "REJECTED",
		TxStatusAcceptedOnL2: "ACCEPTED_ON_L2",
		TxStatusAcceptedOnL1: "ACCEPTED_ON_L1",
	}
	TxStatusValue = map[string]TransactionStatus{
		"UNKNOWN":        TxStatusUnknown,
		"NOT_RECEIVED":   TxStatusNotReceived,
		"RECEIVED":       TxStatusReceived,
		"PENDING":        TxStatusPending,
		"REJECTED":       TxStatusRejected,
		"ACCEPTED_ON_L2": TxStatusAcceptedOnL2,
		"ACCEPTED_ON_L1": TxStatusAcceptedOnL1,
	}
)

func (s TransactionStatus) String() string {
	// notest
	return TxStatusName[s]
}

type TransactionReceipt struct {
	TxHash          *felt.Felt
	ActualFee       *felt.Felt
	Status          TransactionStatus
	StatusData      string
	MessagesSent    []MessageL2ToL1
	L1OriginMessage *MessageL1ToL2
	Events          []Event
}

type TransactionWithReceipt struct {
	IsTransaction
	TransactionReceipt
}
