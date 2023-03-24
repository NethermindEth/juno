package rpc

import "github.com/NethermindEth/juno/core/felt"

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
