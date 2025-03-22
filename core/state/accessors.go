package state

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
)

// Wrapper around getContract which checks if a contract is deployed
func GetContract(r db.KeyValueReader, addr *felt.Felt) (*stateContract, error) {
	contract, err := getContract(r, addr)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, ErrContractNotDeployed
		}
		return nil, err
	}

	return contract, nil
}

// Gets a contract instance from the database.
func getContract(r db.KeyValueReader, addr *felt.Felt) (*stateContract, error) {
	key := db.ContractKey(addr)
	var contract stateContract
	if err := r.Get(key, func(val []byte) error {
		if err := contract.UnmarshalBinary(val); err != nil {
			return fmt.Errorf("failed to unmarshal contract: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return &contract, nil
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
	key := db.ContractHistoryNonceKey(addr, blockNum)
	return w.Put(key, nonce.Marshal())
}

func WriteClassHashHistory(w db.KeyValueWriter, addr *felt.Felt, blockNum uint64, classHash *felt.Felt) error {
	key := db.ContractHistoryClassHashKey(addr, blockNum)
	return w.Put(key, classHash.Marshal())
}

func WriteStorageHistory(w db.KeyValueWriter, addr *felt.Felt, key *felt.Felt, blockNum uint64, value *felt.Felt) error {
	dbKey := db.ContractHistoryStorageKey(addr, key, blockNum)
	return w.Put(dbKey, value.Marshal())
}

func WriteClass(w db.KeyValueWriter, classHash *felt.Felt, class core.Class, declaredAt uint64) error {
	key := db.ClassKey(classHash)

	dc := core.DeclaredClass{
		At:    declaredAt,
		Class: class,
	}
	enc, err := dc.MarshalBinary()
	if err != nil {
		return err
	}

	return w.Put(key, enc)
}

func GetClass(r db.KeyValueReader, classHash *felt.Felt) (*core.DeclaredClass, error) {
	key := db.ClassKey(classHash)

	var class core.DeclaredClass
	if err := r.Get(key, class.UnmarshalBinary); err != nil {
		return nil, err
	}

	return &class, nil
}

func HasClass(r db.KeyValueReader, classHash *felt.Felt) (bool, error) {
	key := db.ClassKey(classHash)
	return r.Has(key)
}
