package main

import "encoding/json"

type blockID interface {
	blockNumber() uint64
}

type blockNumberID struct {
	BlockNumber uint64 `json:"block_number"`
}

func (b blockNumberID) blockNumber() uint64 { return b.BlockNumber }

type blockHashID struct {
	BlockHash string `json:"block_hash"`

	number uint64
}

func (b blockHashID) blockNumber() uint64 { return b.number }

type latestBlockID struct {
	number uint64
}

func (b latestBlockID) blockNumber() uint64 { return b.number }

func (latestBlockID) MarshalJSON() ([]byte, error) {
	return json.Marshal("latest")
}

type blockIDParams struct {
	BlockID       blockID  `json:"block_id"`
	ResponseFlags []string `json:"response_flags,omitempty"`
}

type traceBlockParams struct {
	BlockID    blockID  `json:"block_id"`
	TraceFlags []string `json:"trace_flags,omitempty"`
}

type txHashParams struct {
	TransactionHash string   `json:"transaction_hash"`
	ResponseFlags   []string `json:"response_flags,omitempty"`
}

type txByBlockIDAndIndexParams struct {
	BlockID       blockID  `json:"block_id"`
	Index         uint64   `json:"index"`
	ResponseFlags []string `json:"response_flags,omitempty"`
}

type contractAtBlockParams struct {
	BlockID         blockID `json:"block_id"`
	ContractAddress string  `json:"contract_address"`
}

type classAtBlockParams struct {
	BlockID   blockID `json:"block_id"`
	ClassHash string  `json:"class_hash"`
}

type classHashParams struct {
	ClassHash string `json:"class_hash"`
}

type storageAtParams struct {
	ContractAddress string   `json:"contract_address"`
	Key             string   `json:"key"`
	BlockID         blockID  `json:"block_id"`
	ResponseFlags   []string `json:"response_flags,omitempty"`
}

type eventsParams struct {
	Filter eventFilter `json:"filter"`
}

type eventFilter struct {
	FromBlock blockID     `json:"from_block,omitempty"`
	ToBlock   blockID     `json:"to_block,omitempty"`
	Address   addressList `json:"address,omitempty"`
	Keys      [][]string  `json:"keys,omitempty"`
	ChunkSize uint64      `json:"chunk_size"`
}

type addressList []string

func (a addressList) MarshalJSON() ([]byte, error) {
	if len(a) == 1 {
		return json.Marshal(a[0])
	}
	return json.Marshal([]string(a))
}

type storageProofParams struct {
	BlockID              string                `json:"block_id"`
	ClassHashes          []string              `json:"class_hashes,omitempty"`
	ContractAddresses    []string              `json:"contract_addresses,omitempty"`
	ContractsStorageKeys []contractStorageKeys `json:"contracts_storage_keys,omitempty"`
}

type contractStorageKeys struct {
	ContractAddress string   `json:"contract_address"`
	StorageKeys     []string `json:"storage_keys"`
}

type txHashesBlock struct {
	BlockHash    string   `json:"block_hash"`
	Transactions []string `json:"transactions"`
}

type stateUpdate struct {
	StateDiff stateDiff `json:"state_diff"`
}

type stateDiff struct {
	StorageDiffs []struct {
		Address        string `json:"address"`
		StorageEntries []struct {
			Key string `json:"key"`
		} `json:"storage_entries"`
	} `json:"storage_diffs"`
	Nonces []struct {
		ContractAddress string `json:"contract_address"`
	} `json:"nonces"`
}

type contractClass struct {
	SierraProgram []json.RawMessage `json:"sierra_program"`
}

type receiptEvent struct {
	FromAddress string   `json:"from_address"`
	Keys        []string `json:"keys"`
}

type receiptsBlock struct {
	Transactions []struct {
		Receipt struct {
			Events []receiptEvent `json:"events"`
		} `json:"receipt"`
	} `json:"transactions"`
}
