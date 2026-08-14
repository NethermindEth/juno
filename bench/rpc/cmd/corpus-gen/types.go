package main

import "encoding/json"

type blockNumberID struct {
	BlockNumber uint64 `json:"block_number"`
}

type blockIDParams struct {
	BlockID blockNumberID `json:"block_id"`
}

type txHashParams struct {
	TransactionHash string `json:"transaction_hash"`
}

type txByBlockIDAndIndexParams struct {
	BlockID blockNumberID `json:"block_id"`
	Index   uint64        `json:"index"`
}

type contractAtBlockParams struct {
	BlockID         blockNumberID `json:"block_id"`
	ContractAddress string        `json:"contract_address"`
}

type classAtBlockParams struct {
	BlockID   blockNumberID `json:"block_id"`
	ClassHash string        `json:"class_hash"`
}

type classHashParams struct {
	ClassHash string `json:"class_hash"`
}

type storageAtParams struct {
	ContractAddress string        `json:"contract_address"`
	Key             string        `json:"key"`
	BlockID         blockNumberID `json:"block_id"`
}

type eventsParams struct {
	Filter eventFilter `json:"filter"`
}

type eventFilter struct {
	FromBlock blockNumberID `json:"from_block"`
	ToBlock   blockNumberID `json:"to_block"`
	Address   string        `json:"address,omitempty"`
	ChunkSize int           `json:"chunk_size"`
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

type receiptsBlock struct {
	Transactions []struct {
		Receipt struct {
			Events []struct {
				FromAddress string `json:"from_address"`
			} `json:"events"`
		} `json:"receipt"`
	} `json:"transactions"`
}
