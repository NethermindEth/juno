package rpc

import (
	"encoding/json"
	"errors"

	"github.com/NethermindEth/juno/core/felt"
)

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1999-L2008
type Status uint8

const (
	StatusPending Status = iota
	StatusAcceptedL2
	StatusAcceptedL1
	StatusRejected
)

func (s Status) MarshalJSON() ([]byte, error) {
	switch s {
	case StatusPending:
		return []byte("\"PENDING\""), nil
	case StatusAcceptedL2:
		return []byte("\"ACCEPTED_ON_L2\""), nil
	case StatusAcceptedL1:
		return []byte("\"ACCEPTED_ON_L1\""), nil
	case StatusRejected:
		return []byte("\"REJECTED\""), nil
	default:
		return nil, errors.New("unknown block status")
	}
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L520-L534
type BlockNumberAndHash struct {
	Number uint64     `json:"block_number"`
	Hash   *felt.Felt `json:"block_hash"`
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L814
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

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1072
type BlockHeader struct {
	Hash             *felt.Felt `json:"block_hash"`
	ParentHash       *felt.Felt `json:"parent_hash"`
	Number           uint64     `json:"block_number"`
	NewRoot          *felt.Felt `json:"new_root"`
	Timestamp        uint64     `json:"timestamp"`
	SequencerAddress *felt.Felt `json:"sequencer_address,omitempty"`
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1131
type BlockWithTxs struct {
	Status Status `json:"status"`
	BlockHeader
	Transactions []*Transaction `json:"transactions"`
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1109
type BlockWithTxHashes struct {
	Status Status `json:"status"`
	BlockHeader
	TxnHashes []*felt.Felt `json:"transactions"`
}

// https://github.com/starkware-libs/starknet-specs/blob/ed85de6a701c966ec7395b0df4c8bd62f3cddbcd/api/starknet_api_openrpc.json#L844
type StateUpdate struct {
	BlockHash *felt.Felt `json:"block_hash"`
	NewRoot   *felt.Felt `json:"new_root"`
	OldRoot   *felt.Felt `json:"old_root"`
	StateDiff *StateDiff `json:"state_diff"`
}

type StateDiff struct {
	StorageDiffs      []StorageDiff      `json:"storage_diffs"`
	Nonces            []Nonce            `json:"nonces"`
	DeployedContracts []DeployedContract `json:"deployed_contracts"`
	DeclaredClasses   []*felt.Felt       `json:"declared_contract_hashes"`
}

type Nonce struct {
	ContractAddress *felt.Felt `json:"contract_address"`
	Nonce           *felt.Felt `json:"nonce"`
}

type StorageDiff struct {
	Address        *felt.Felt `json:"address"`
	StorageEntries []Entry    `json:"storage_entries"`
}

type Entry struct {
	Key   *felt.Felt `json:"key"`
	Value *felt.Felt `json:"value"`
}

type DeployedContract struct {
	Address   *felt.Felt `json:"address"`
	ClassHash *felt.Felt `json:"class_hash"`
}
