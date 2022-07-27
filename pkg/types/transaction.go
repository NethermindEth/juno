package types

import (
	"github.com/NethermindEth/juno/pkg/felt"
)

type IsTransaction interface {
	GetHash() *felt.Felt
}

type TransactionDeploy struct {
	Hash                *felt.Felt
	ClassHash           *felt.Felt
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

type TxnStatus int64

const (
	TxStatusUnknown TxnStatus = iota
	TxStatusNotReceived
	TxStatusReceived
	TxStatusPending
	TxStatusRejected
	TxStatusAcceptedOnL2
	TxStatusAcceptedOnL1
)

var (
	TxStatusName = map[TxnStatus]string{
		TxStatusUnknown:      "UNKNOWN",
		TxStatusNotReceived:  "NOT_RECEIVED",
		TxStatusReceived:     "RECEIVED",
		TxStatusPending:      "PENDING",
		TxStatusRejected:     "REJECTED",
		TxStatusAcceptedOnL2: "ACCEPTED_ON_L2",
		TxStatusAcceptedOnL1: "ACCEPTED_ON_L1",
	}
	TxStatusValue = map[string]TxnStatus{
		"UNKNOWN":        TxStatusUnknown,
		"NOT_RECEIVED":   TxStatusNotReceived,
		"RECEIVED":       TxStatusReceived,
		"PENDING":        TxStatusPending,
		"REJECTED":       TxStatusRejected,
		"ACCEPTED_ON_L2": TxStatusAcceptedOnL2,
		"ACCEPTED_ON_L1": TxStatusAcceptedOnL1,
	}
)

func (s TxnStatus) String() string {
	// notest
	return TxStatusName[s]
}

type TxnReceipt interface {
	isReceipt()
}

type TxnReceiptCommon struct {
	TxnHash     *felt.Felt
	ActualFee   *felt.Felt
	Status      TxnStatus
	StatusData  string
	BlockHash   *felt.Felt
	BlockNumber uint64
}

func (*TxnReceiptCommon) isReceipt() {}

type TxnInvokeReceipt struct {
	TxnReceiptCommon
	MessagesSent    []*MsgToL1
	L1OriginMessage *MsgToL2
	Events          []*Event
}

type TxnDeclareReceipt struct {
	TxnReceiptCommon
}

type TxnDeployReceipt struct {
	TxnReceiptCommon
}
