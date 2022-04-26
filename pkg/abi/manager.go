package abi

import (
	"encoding/json"
	"fmt"
	"github.com/NethermindEth/juno/pkg/db"
)

type Manager struct {
	database db.Databaser
}

func NewABIManager(database db.Databaser) *Manager {
	return &Manager{database}
}

func (m *Manager) GetABI(contractAddress string, blockNumber uint64) (*Abi, error) {
	// Get the sortedList of block version
	data, err := m.database.Get(newSimpleKey(contractAddress))
	if err != nil {
		return nil, err
	}
	s := new(sortedList)
	if err := json.Unmarshal(data, s); err != nil {
		return nil, err
	}
	// Find the best block to fit
	bestFit, ok := s.Search(blockNumber)
	if !ok {
		// notest
		return nil, fmt.Errorf("ABI for %s at block %d does not exists", contractAddress, blockNumber)
	}
	// Get the ABI
	data, err = m.database.Get(newCompoundedKey(contractAddress, bestFit))
	if err != nil {
		return nil, err
	}
	abi := new(Abi)
	if err := json.Unmarshal(data, abi); err != nil {
		return nil, err
	}
	return abi, nil
}

func (m *Manager) putSortedList(key []byte, x sortedList) error {
	data, err := json.Marshal(&x)
	if err != nil {
		return err
	}
	err = m.database.Put(key, data)
	if err != nil {
		return err
	}
	return nil
}

func (m *Manager) PutABI(contractAddress string, blockNumber uint64, abi *Abi) (err error) {
	// TODO: This operation must be done with a DB transaction
	key := newSimpleKey(contractAddress)
	ok, err := m.database.Has(key)
	if err != nil || !ok {
		err = m.putSortedList(key, []uint64{blockNumber})
		if err != nil {
			return err
		}
	} else {
		data, err := m.database.Get(key)
		if err != nil {
			return err
		}
		s := new(sortedList)
		err = json.Unmarshal(data, s)
		if err != nil {
			return err
		}
		s.Add(blockNumber)
		data, err = json.Marshal(s)
		if err != nil {
			return err
		}
		err = m.database.Put(key, data)
		if err != nil {
			return err
		}
	}
	data, err := json.Marshal(abi)
	if err != nil {
		return err
	}
	err = m.database.Put(newCompoundedKey(contractAddress, blockNumber), data)
	if err != nil {
		return err
	}
	return nil
}
