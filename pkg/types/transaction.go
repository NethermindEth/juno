package types

type IsTransaction interface {
	GetHash() PedersenHash
}

type TransactionDeploy struct {
	Hash                PedersenHash
	ContractAddress     Address
	ConstructorCallData []Felt
}

func (tx *TransactionDeploy) GetHash() PedersenHash {
	return tx.Hash
}

type TransactionInvoke struct {
	Hash               PedersenHash `json:"txn_hash"`
	ContractAddress    Address      `json:"contract_address"`
	EntryPointSelector Felt         `json:"entry_point_selector"`
	CallData           []Felt       `json:"calldata"`
	Signature          []Felt       `json:"-"`
	MaxFee             Felt         `json:"max_fee"`
}

func (tx *TransactionInvoke) GetHash() PedersenHash {
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
	TxHash          PedersenHash
	ActualFee       Felt
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
