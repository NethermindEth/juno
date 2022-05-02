package state

import (
	"encoding/json"
	"fmt"
	"math/big"
)

type ContractAddress string

func (x ContractAddress) Marshal() ([]byte, error) {
	i, ok := new(big.Int).SetString(string(x), 16)
	if !ok {
		return nil, InvalidContractAddress
	}
	return i.Bytes(), nil
}

type contractStorageItem struct {
	Key   big.Int
	Value big.Int
}

type ContractStorage []contractStorageItem

func (s *ContractStorage) Marshal() ([]byte, error) {
	data := make(map[string]string)
	for _, item := range *s {
		data["0x"+item.Key.Text(16)] = "0x" + item.Value.Text(16)
	}
	return json.Marshal(data)
}

func (s *ContractStorage) Unmarshal(data []byte) error {
	m := make(map[string]string)
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	storage := make([]contractStorageItem, 0, len(m))
	for k, v := range m {
		key, ok := new(big.Int).SetString(k[2:], 16)
		if !ok {
			return fmt.Errorf("error parsing %s[2:] into an big.Int of base 16", k)
		}
		value, ok := new(big.Int).SetString(v[2:], 16)
		if !ok {
			return fmt.Errorf("error parsing %s[2:] into an big.Int of base 16", v)
		}
		storage = append(storage, contractStorageItem{*key, *value})
	}
	*s = storage
	return nil
}

func (x *Manager) GetStorage(contractAddress string, blockNumber uint64) (*ContractStorage, bool) {
	var value ContractStorage
	ok := x.storageDatabase.Get(ContractAddress(contractAddress), blockNumber, &value)
	if !ok {
		return nil, false
	}
	return &value, true
}

func (x *Manager) PutStorage(contractAddress string, blockNumber uint64, storage *ContractStorage) {
	x.storageDatabase.Put(ContractAddress(contractAddress), blockNumber, storage)
}
