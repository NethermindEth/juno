package types

import (
	"encoding/json"

	"github.com/NethermindEth/juno/pkg/felt"
)

type BlockStatus int32

const (
	BlockStatusUnknown BlockStatus = iota
	BlockStatusPending
	BlockStatusProven
	BlockStatusAcceptedOnL2
	BlockStatusAcceptedOnL1
	BlockStatusRejected
)

var (
	BlockStatusName = map[BlockStatus]string{
		BlockStatusUnknown:      "UNKNOWN",
		BlockStatusPending:      "PENDING",
		BlockStatusProven:       "PROVEN",
		BlockStatusAcceptedOnL2: "ACCEPTED_ON_L2",
		BlockStatusAcceptedOnL1: "ACCEPTED_ON_L1",
		BlockStatusRejected:     "REJECTED",
	}
	BlockStatusValue = map[string]BlockStatus{
		"UNKNOWN":        BlockStatusUnknown,
		"PENDING":        BlockStatusPending,
		"PROVEN":         BlockStatusProven,
		"ACCEPTED_ON_L2": BlockStatusAcceptedOnL2,
		"ACCEPTED_ON_L1": BlockStatusAcceptedOnL1,
		"REJECTED":       BlockStatusRejected,
	}
)

func StringToBlockStatus(s string) BlockStatus {
	blockStatus, ok := BlockStatusValue[s]
	if !ok {
		// notest
		return BlockStatusUnknown
	}
	return blockStatus
}

func (b BlockStatus) String() string {
	return BlockStatusName[b]
}

// notest
func (b BlockStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(b.String())
}

type BlockTag string

type Block struct {
	BlockHash    *felt.Felt  `json:"bloch_hash"`
	ParentHash   *felt.Felt  `json:"parent_hash"`
	BlockNumber  uint64      `json:"block_number"`
	Status       BlockStatus `json:"status"`
	Sequencer    *felt.Felt  `json:"sequencer"`
	NewRoot      *felt.Felt  `json:"new_root,omitempty"`
	OldRoot      *felt.Felt  `json:"old_root"`
	AcceptedTime int64       `json:"accepted_time"`
	TimeStamp    int64       `json:"time_stamp"`

	TxCount      uint64     `json:"tx_count"`
	TxCommitment *felt.Felt `json:"tx_commitment"`
	TxHashes     []*felt.Felt

	EventCount      uint64     `json:"event_count"`
	EventCommitment *felt.Felt `json:"event_commitment"`
}
