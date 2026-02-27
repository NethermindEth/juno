package rpcv10

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
)

type SubscriptionResponse struct {
	Version string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

// As per the spec, this is the same as BlockID, but without `pre_confirmed` and `l1_accepted`
type SubscriptionBlockID BlockID

func (b *SubscriptionBlockID) Type() blockIDType {
	return b.typeID
}

func (b *SubscriptionBlockID) IsLatest() bool {
	return b.typeID == latest
}

func (b *SubscriptionBlockID) IsHash() bool {
	return b.typeID == hash
}

func (b *SubscriptionBlockID) IsNumber() bool {
	return b.typeID == number
}

func (b *SubscriptionBlockID) Hash() *felt.Felt {
	return (*BlockID)(b).Hash()
}

func (b *SubscriptionBlockID) Number() uint64 {
	return (*BlockID)(b).Number()
}

func (b *SubscriptionBlockID) UnmarshalJSON(data []byte) error {
	blockID := (*BlockID)(b)
	err := blockID.UnmarshalJSON(data)
	if err != nil {
		return err
	}

	if blockID.IsPreConfirmed() {
		return errors.New("subscription block id cannot be pre_confirmed")
	}

	if blockID.IsL1Accepted() {
		return errors.New("subscription block id cannot be l1_accepted")
	}

	return nil
}

type ReorgEvent struct {
	StartBlockHash *felt.Felt `json:"starting_block_hash"`
	StartBlockNum  uint64     `json:"starting_block_number"`
	EndBlockHash   *felt.Felt `json:"ending_block_hash"`
	EndBlockNum    uint64     `json:"ending_block_number"`
}

type TxnFinalityStatusWithoutL1 TxnFinalityStatus

func (s *TxnFinalityStatusWithoutL1) UnmarshalText(text []byte) error {
	var base TxnFinalityStatus
	if err := base.UnmarshalText(text); err != nil {
		return err
	}
	// Validate that only non-L1 statuses are allowed
	if base == TxnAcceptedOnL1 {
		return fmt.Errorf("invalid TxnStatus: %s;", text)
	}
	*s = TxnFinalityStatusWithoutL1(base)
	return nil
}

func (s TxnFinalityStatusWithoutL1) MarshalText() ([]byte, error) {
	switch s {
	case TxnFinalityStatusWithoutL1(TxnPreConfirmed):
		return []byte("PRE_CONFIRMED"), nil
	case TxnFinalityStatusWithoutL1(TxnAcceptedOnL2):
		return []byte("ACCEPTED_ON_L2"), nil
	default:
		return nil, fmt.Errorf("unknown TxnFinalityStatusWithoutL1 %v", s)
	}
}

type TxnStatusWithoutL1 TxnStatus

func (s *TxnStatusWithoutL1) UnmarshalText(text []byte) error {
	switch string(text) {
	case "RECEIVED":
		*s = TxnStatusWithoutL1(TxnStatusReceived)
		return nil
	case "CANDIDATE":
		*s = TxnStatusWithoutL1(TxnStatusCandidate)
		return nil
	case "PRE_CONFIRMED":
		*s = TxnStatusWithoutL1(TxnStatusPreConfirmed)
		return nil
	case "ACCEPTED_ON_L2":
		*s = TxnStatusWithoutL1(TxnStatusAcceptedOnL2)
		return nil
	default:
		return fmt.Errorf("invalid TxnStatus: %s;", text)
	}
}

func (s TxnStatusWithoutL1) MarshalText() ([]byte, error) {
	switch s {
	case TxnStatusWithoutL1(TxnStatusReceived):
		return []byte("RECEIVED"), nil
	case TxnStatusWithoutL1(TxnStatusCandidate):
		return []byte("CANDIDATE"), nil
	case TxnStatusWithoutL1(TxnStatusPreConfirmed):
		return []byte("PRE_CONFIRMED"), nil
	case TxnStatusWithoutL1(TxnStatusAcceptedOnL2):
		return []byte("ACCEPTED_ON_L2"), nil
	default:
		return nil, fmt.Errorf("unknown TxnStatusWithoutL1 %v", s)
	}
}

type SubscriptionTransactionStatus struct {
	TransactionHash *felt.Felt        `json:"transaction_hash"`
	Status          TransactionStatus `json:"status"`
}

type SentReceipt struct {
	TransactionHash  felt.Felt
	TransactionIndex int
}
