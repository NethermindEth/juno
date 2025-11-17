package starknet

import "github.com/NethermindEth/juno/core/felt"

// StateUpdate object returned by the feeder in JSON format for "get_state_update" endpoint
type StateUpdate struct {
	BlockHash *felt.Felt `json:"block_hash"`
	NewRoot   *felt.Felt `json:"new_root"`
	OldRoot   *felt.Felt `json:"old_root"`
	StateDiff StateDiff  `json:"state_diff"`
}

type StateDiff struct {
	// todo(rdr): What is key and value, I think it should go with `felt.StorageKey` and
	//            `felt.StorageValue`. Also, why pointers to values and not values directly
	//             Also, doesn't make complete sense right now why the key is a string instead of
	//             felt.Address
	StorageDiffs map[string][]struct {
		Key   *felt.Felt `json:"key"`
		Value *felt.Felt `json:"value"`
	} `json:"storage_diffs"`

	// todo(rdr): `felt.Nonce` or maybe uint64, it doesn't make sense otherwise
	//             Also, doesn't make complete sense right now why the key is a string instead of
	//             felt.Address
	Nonces map[string]*felt.Felt `json:"nonces"`

	// todo(rdr): change felt.Felt for felt.Address and felt.SierraClassHash. Also, should it be
	//            pointers to values, or values directly
	DeployedContracts []struct {
		Address   *felt.Felt `json:"address"`
		ClassHash *felt.Felt `json:"class_hash"`
	} `json:"deployed_contracts"`

	// v0.11.0
	// todo(rdr): is "old declared contracts" deprecated cairo contracts? We might need a
	//            a `felt.DeprecatedCairoClassHash`
	OldDeclaredContracts []*felt.Felt `json:"old_declared_contracts"`
	// todo(rdr): change the felt here to be `felt.SierraClassHash` and `felt.CasmClassHash`
	DeclaredClasses []struct {
		ClassHash         *felt.Felt `json:"class_hash"`
		CompiledClassHash *felt.Felt `json:"compiled_class_hash"`
	} `json:"declared_classes"`
	// todo(rdr): change the felts here to be `felt.Address` and `felt.SierraClassHash`
	//            but if ClassHash may include other class hashes (deprecated class hashes) then
	//            it is probably better to have `felt.ClassHash`
	ReplacedClasses []struct {
		Address   *felt.Felt `json:"address"`
		ClassHash *felt.Felt `json:"class_hash"`
	} `json:"replaced_classes"`
	MigratedClasses []struct {
		ClassHash         felt.SierraClassHash `json:"class_hash"`
		CompiledClassHash felt.CasmClassHash   `json:"compiled_class_hash"`
	} `json:"migrated_compiled_classes"`
}

// StateUpdateWithBlock object returned by the feeder in JSON format for "get_state_update" endpoint with includingBlock arg
type StateUpdateWithBlock struct {
	Block       *Block       `json:"block"`
	StateUpdate *StateUpdate `json:"state_update"`
}
