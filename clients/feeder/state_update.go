package feeder

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
		DeclaredContracts []*felt.Felt `json:"declared_contracts"`
	} `json:"state_diff"`
}
