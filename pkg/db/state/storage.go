package state

import (
	"encoding/json"
	"math/big"
)

// ContractAddress represents the contract address used
// as a key in the database
type ContractAddress string

func (x ContractAddress) Marshal() ([]byte, error) {
	i, ok := new(big.Int).SetString(string(x), 16)
	if !ok {
		// notest
		return nil, InvalidContractAddress
	}
	return i.Bytes(), nil
}

// ContractStorage is the representation of the StarkNet contract
// storage.
type ContractStorage map[string]string

func (s *ContractStorage) Marshal() ([]byte, error) {
	return json.Marshal(s)
}

func (s *ContractStorage) Unmarshal(data []byte) error {
	m := make(map[string]string)
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	*s = m
	return nil
}

// GetStorage returns the ContractStorage state of the given contract address and block number.
// If no exists a version for exactly the given block number, then returns the newest version
// lower than the given block number. If the contract storage does not exists then returns nil.
func (x *Manager) GetStorage(contractAddress string, blockNumber uint64) *ContractStorage {
	var value ContractStorage
	ok := x.storageDatabase.Get(ContractAddress(contractAddress), blockNumber, &value)
	if !ok {
		return nil
	}
	return &value
}

// PutStorage saves a new version of the contract storage at the given block number.
func (x *Manager) PutStorage(contractAddress string, blockNumber uint64, storage *ContractStorage) {
	x.storageDatabase.Put(ContractAddress(contractAddress), blockNumber, storage)
}
