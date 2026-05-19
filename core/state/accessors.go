package state

import (
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
)

func GetStateObject(r db.KeyValueReader, state *State, addr *felt.Felt) (*stateObject, error) {
	contract, err := GetContract(r, addr)
	if err != nil {
		return nil, err
	}

	obj := newStateObject(state, addr, &contract)
	return &obj, nil
}

// Gets a contract instance from the database.
// Returns db.ErrKeyNotFound when the contract does not exist, for backward compatibility.
// TODO: return more precise error (e.g. ErrContractNotDeployed) once deprecated state is removed.
func GetContract(r db.KeyValueReader, addr *felt.Felt) (stateContract, error) {
	key := db.ContractKey(addr)
	var contract stateContract
	if err := r.Get(key, func(val []byte) error {
		if err := contract.UnmarshalBinary(val); err != nil {
			return fmt.Errorf("failed to unmarshal contract: %w", err)
		}
		return nil
	}); err != nil {
		return stateContract{}, err
	}
	return contract, nil
}

// todo(rdr): `addr` should be a felt.Addr
func HasContract(r db.KeyValueReader, addr *felt.Felt) (bool, error) {
	key := db.ContractKey(addr)
	return r.Has(key)
}

// WriteContract writes a Contract record from raw fields. Used by the running
// node (via writeContract on a fully-built stateContract) and by the deprecated
// → new state migration (with StorageRoot left zero — the new state lazily
// backfills it on the contract's first storage write).
func WriteContract(
	w db.KeyValueWriter,
	addr *felt.Felt,
	nonce, classHash felt.Felt,
	deployHeight uint64,
) error {
	contract := stateContract{
		Nonce:          nonce,
		ClassHash:      classHash,
		DeployedHeight: deployHeight,
	}
	return writeContract(w, addr, &contract)
}

func writeContract(w db.KeyValueWriter, addr *felt.Felt, contract *stateContract) error {
	key := db.ContractKey(addr)
	data, err := contract.MarshalBinary()
	if err != nil {
		return err
	}

	return w.Put(key, data)
}

func DeleteContract(w db.KeyValueWriter, addr *felt.Felt) error {
	key := db.ContractKey(addr)
	return w.Delete(key)
}

func WriteNonceHistory(w db.KeyValueWriter, addr *felt.Felt, blockNum uint64, nonce *felt.Felt) error {
	key := db.ContractNonceHistoryAtBlockKey(addr, blockNum)
	return w.Put(key, nonce.Marshal())
}

func WriteClassHashHistory(w db.KeyValueWriter, addr *felt.Felt, blockNum uint64, classHash *felt.Felt) error {
	key := db.ContractClassHashHistoryAtBlockKey(addr, blockNum)
	return w.Put(key, classHash.Marshal())
}

func WriteStorageHistory(w db.KeyValueWriter, addr, key *felt.Felt, blockNum uint64, value *felt.Felt) error {
	dbKey := db.ContractStorageHistoryAtBlockKey(addr, key, blockNum)
	return w.Put(dbKey, value.Marshal())
}

func DeleteStorageHistory(w db.KeyValueWriter, addr, key *felt.Felt, blockNum uint64) error {
	dbKey := db.ContractStorageHistoryAtBlockKey(addr, key, blockNum)
	return w.Delete(dbKey)
}

func DeleteNonceHistory(w db.KeyValueWriter, addr *felt.Felt, blockNum uint64) error {
	dbKey := db.ContractNonceHistoryAtBlockKey(addr, blockNum)
	return w.Delete(dbKey)
}

func DeleteClassHashHistory(w db.KeyValueWriter, addr *felt.Felt, blockNum uint64) error {
	dbKey := db.ContractClassHashHistoryAtBlockKey(addr, blockNum)
	return w.Delete(dbKey)
}

func WriteClass(
	w db.KeyValueWriter,
	classHash *felt.Felt,
	class *core.DeclaredClassDefinition,
) error {
	enc, err := encoder.Marshal(class)
	if err != nil {
		return err
	}
	return w.Put(db.ClassKey(classHash), enc)
}

func DeleteClass(w db.KeyValueWriter, classHash *felt.Felt) error {
	key := db.ClassKey(classHash)
	return w.Delete(key)
}

func GetClass(r db.KeyValueReader, classHash *felt.Felt) (*core.DeclaredClassDefinition, error) {
	key := db.ClassKey(classHash)

	var class core.DeclaredClassDefinition
	if err := r.Get(key, func(data []byte) error {
		return encoder.Unmarshal(data, &class)
	}); err != nil {
		return nil, err
	}

	return &class, nil
}

func HasClass(r db.KeyValueReader, classHash *felt.Felt) (bool, error) {
	key := db.ClassKey(classHash)
	return r.Has(key)
}
