package rpc

import (
	"encoding/json"
	"errors"

	"github.com/NethermindEth/juno/core/felt"
)

type BlockNumberAndHash struct {
	Number uint64     `json:"block_number"`
	Hash   *felt.Felt `json:"block_hash"`
}

type BlockId struct {
	Pending bool
	Latest  bool
	Hash    *felt.Felt
	Number  uint64
}

func (b *BlockId) UnmarshalJSON(data []byte) error {
	if "\"latest\"" == string(data) {
		b.Latest = true
	} else if "\"pending\"" == string(data) {
		b.Pending = true
	} else {
		jsonObject := make(map[string]json.RawMessage)
		if err := json.Unmarshal(data, &jsonObject); err != nil {
			return err
		} else {
			hash, ok := jsonObject["block_hash"]
			if ok {
				b.Hash = new(felt.Felt)
				return json.Unmarshal(hash, b.Hash)
			}

			number, ok := jsonObject["block_number"]
			if ok {
				return json.Unmarshal(number, &b.Number)
			}

			return errors.New("cannot unmarshal block id")
		}
	}
	return nil
}

type BlockStatus uint8

const (
	BlockStatusPending BlockStatus = iota
	BlockStatusAcceptedL2
	BlockStatusAcceptedL1
	BlockStatusRejected
)

func (s BlockStatus) MarshalJSON() ([]byte, error) {
	switch s {
	case BlockStatusPending:
		return []byte("\"PENDING\""), nil
	case BlockStatusAcceptedL2:
		return []byte("\"ACCEPTED_ON_L2\""), nil
	case BlockStatusAcceptedL1:
		return []byte("\"ACCEPTED_ON_L1\""), nil
	case BlockStatusRejected:
		return []byte("\"REJECTED\""), nil
	default:
		return nil, errors.New("unknown block status")
	}
}

type BlockHeader struct {
	Hash             *felt.Felt `json:"block_hash"`
	ParentHash       *felt.Felt `json:"parent_hash"`
	Number           uint64     `json:"block_number"`
	NewRoot          *felt.Felt `json:"new_root"`
	Timestamp        uint64     `json:"timestamp"`
	SequencerAddress *felt.Felt `json:"sequencer_address,omitempty"`
}

type BlockWithTxHashes struct {
	Status BlockStatus `json:"status"`
	BlockHeader
	TxnHashes []*felt.Felt `json:"transactions"`
}
