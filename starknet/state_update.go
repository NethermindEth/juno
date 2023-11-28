package starknet

import "github.com/NethermindEth/juno/core/felt"

// StateUpdate object returned by the feeder in JSON format for "get_state_update" endpoint
type StateUpdate struct {
	BlockHash *felt.Felt `json:"block_hash"`
	NewRoot   *felt.Felt `json:"new_root"`
	OldRoot   *felt.Felt `json:"old_root"`

	StateDiff struct {
		StorageDiffs map[string][]struct {
			Key   *felt.Felt `json:"key"`
			Value *felt.Felt `json:"value"`
		} `json:"storage_diffs"`

		Nonces            map[string]*felt.Felt `json:"nonces"`
		DeployedContracts []struct {
			Address   *felt.Felt `json:"address"`
			ClassHash *felt.Felt `json:"class_hash"`
		} `json:"deployed_contracts"`

		// v0.11.0
		OldDeclaredContracts []*felt.Felt `json:"old_declared_contracts"`
		DeclaredClasses      []struct {
			ClassHash         *felt.Felt `json:"class_hash"`
			CompiledClassHash *felt.Felt `json:"compiled_class_hash"`
		} `json:"declared_classes"`
		ReplacedClasses []struct {
			Address   *felt.Felt `json:"address"`
			ClassHash *felt.Felt `json:"class_hash"`
		} `json:"replaced_classes"`
	} `json:"state_diff"`
}

// StateUpdateWithBlock object returned by the feeder in JSON format for "get_state_update" endpoint with includingBlock arg
type StateUpdateWithBlock struct {
	Block       *Block       `json:"block"`
	StateUpdate *StateUpdate `json:"state_update"`
}
