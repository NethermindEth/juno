package starknet

import "github.com/NethermindEth/juno/core/felt"

// StateUpdate object returned by the feeder in JSON format for "get_state_update" endpoint
type StateUpdate struct {
	BlockHash *felt.Felt          `json:"block_hash"`
	NewRoot   *felt.StateRootHash `json:"new_root"`
	OldRoot   *felt.StateRootHash `json:"old_root"`
	StateDiff StateDiff           `json:"state_diff"`
}

// TODO: placeholder for now to avoid compiler errors. A proper validation
// should be implemented in a follow-up PR.
func (val *StateUpdate) Validate() error {
	return nil
}

type StateDiff struct {
	// todo(rdr): What is key and value, I think it should go with `felt.StorageKey` and
	//            `felt.StorageValue`. Also, why pointers to values and not values directly
	StorageDiffs map[felt.Address][]struct {
		Key   *felt.Felt `json:"key"`
		Value *felt.Felt `json:"value"`
	} `json:"storage_diffs"`

	// todo(rdr): `felt.Nonce` or maybe uint64, it doesn't make sense otherwise
	Nonces map[felt.Address]*felt.Felt `json:"nonces"`

	// todo(rdr): should it be pointers to values, or values directly
	DeployedContracts []struct {
		Address   *felt.Address   `json:"address"`
		ClassHash *felt.ClassHash `json:"class_hash"`
	} `json:"deployed_contracts"`

	// v0.11.0
	// todo(rdr): is "old declared contracts" deprecated cairo contracts? We might need a
	//            a `felt.DeprecatedCairoClassHash`
	OldDeclaredContracts []*felt.ClassHash `json:"old_declared_contracts"`
	DeclaredClasses      []struct {
		ClassHash         *felt.SierraClassHash `json:"class_hash"`
		CompiledClassHash *felt.CasmClassHash   `json:"compiled_class_hash"`
	} `json:"declared_classes"`
	ReplacedClasses []struct {
		Address   *felt.Address   `json:"address"`
		ClassHash *felt.ClassHash `json:"class_hash"`
	} `json:"replaced_classes"`
	MigratedClasses []struct {
		ClassHash         felt.SierraClassHash `json:"class_hash"`
		CompiledClassHash felt.CasmClassHash   `json:"compiled_class_hash"`
	} `json:"migrated_compiled_classes"`
}

// StateUpdateWithBlockAndSignature represents the object response by the
// feeder for "get_state_update" endpoint with `includeBlock` and
// `includeSignature` arguments
type StateUpdateWithBlockAndSignature struct {
	StateUpdate *StateUpdate `json:"state_update"`
	Block       *Block       `json:"block"`
	Signature   []*felt.Felt `json:"signature"`
}

// TODO: placeholder for now to avoid compiler errors. A proper validation
// should be implemented in a follow-up PR.
func (val *StateUpdateWithBlockAndSignature) Validate() error {
	return nil
}
