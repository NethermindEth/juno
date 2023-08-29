package rpc

import "github.com/NethermindEth/juno/core/felt"

// https://github.com/starkware-libs/starknet-specs/blob/8016dd08ed7cd220168db16f24c8a6827ab88317/api/starknet_api_openrpc.json#L909
type StateUpdate struct {
	BlockHash *felt.Felt `json:"block_hash,omitempty"`
	NewRoot   *felt.Felt `json:"new_root,omitempty"`
	OldRoot   *felt.Felt `json:"old_root"`
	StateDiff *StateDiff `json:"state_diff"`
}

type StateDiff struct {
	StorageDiffs              []StorageDiff      `json:"storage_diffs"`
	Nonces                    []Nonce            `json:"nonces"`
	DeployedContracts         []DeployedContract `json:"deployed_contracts"`
	DeprecatedDeclaredClasses []*felt.Felt       `json:"deprecated_declared_classes"`
	DeclaredClasses           []DeclaredClass    `json:"declared_classes"`
	ReplacedClasses           []ReplacedClass    `json:"replaced_classes"`
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

type ReplacedClass struct {
	ContractAddress *felt.Felt `json:"contract_address"`
	ClassHash       *felt.Felt `json:"class_hash"`
}

type DeclaredClass struct {
	ClassHash         *felt.Felt `json:"class_hash"`
	CompiledClassHash *felt.Felt `json:"compiled_class_hash"`
}
