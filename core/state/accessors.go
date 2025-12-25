package state

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
)

func GetStateObject(r db.KeyValueReader, state *State, addr *felt.Felt) (*stateObject, error) {
	contract, err := GetContract(r, addr)
	if err != nil {
		return nil, err
	}

	obj := newStateObject(state, addr, &contract)
	return &obj, nil
}

// Gets a contract instance from the database. If it doesn't exist returns a contract not deployed error
func GetContract(r db.KeyValueReader, addr *felt.Felt) (stateContract, error) {
	contract, err := getContract(r, addr)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return stateContract{}, ErrContractNotDeployed
		}
		return stateContract{}, err
	}

	return contract, nil
}

// Gets a contract instance from the database.
func getContract(r db.KeyValueReader, addr *felt.Felt) (stateContract, error) {
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

func HasContract(r db.KeyValueReader, addr *felt.Felt) (bool, error) {
	key := db.ContractKey(addr)
	return r.Has(key)
}

func WriteContract(w db.KeyValueWriter, addr *felt.Felt, contract *stateContract) error {
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
	class core.ClassDefinition,
	declaredAt uint64,
) error {
	key := db.ClassKey(classHash)

	dc := core.DeclaredClassDefinition{
		At:    declaredAt,
		Class: class,
	}
	enc, err := dc.MarshalBinary()
	if err != nil {
		return err
	}

	return w.Put(key, enc)
}

func DeleteClass(w db.KeyValueWriter, classHash *felt.Felt) error {
	key := db.ClassKey(classHash)
	return w.Delete(key)
}

func GetClass(r db.KeyValueReader, classHash *felt.Felt) (*core.DeclaredClassDefinition, error) {
	key := db.ClassKey(classHash)

	var class core.DeclaredClassDefinition
	if err := r.Get(key, class.UnmarshalBinary); err != nil {
		return nil, err
	}

	return &class, nil
}

func HasClass(r db.KeyValueReader, classHash *felt.Felt) (bool, error) {
	key := db.ClassKey(classHash)
	return r.Has(key)
}
