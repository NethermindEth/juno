package state

import (
	"fmt"
	"google.golang.org/protobuf/proto"
)

func (s *Storage) Update(other *Storage) {
	if s.Storage == nil {
		s.Storage = make(map[string]string)
	}
	// notest
	for key, value := range other.Storage {
		s.Storage[key] = value
	}
}

// GetStorage returns the ContractStorage state of the given contract address
// and block number. If no exists a version for exactly the given block number,
// then returns the newest version lower than the given block number. If the
// contract storage does not exist then returns nil.
func (x *Manager) GetStorage(contractAddress string, blockNumber uint64) *Storage {
	rawData, err := x.storageDatabase.Get([]byte(contractAddress), blockNumber)
	if err != nil {
		panic(any(fmt.Errorf("database error: %s", err)))
	}
	value := new(Storage)
	err = proto.Unmarshal(rawData, value)
	if err != nil {
		panic(any(fmt.Errorf("unmarshal error: %s", err)))
	}
	return value
}

// PutStorage saves a new version of the contract storage at the given block
// number.
func (x *Manager) PutStorage(contractAddress string, blockNumber uint64, storage *Storage) {
	rawValue, err := proto.Marshal(storage)
	if err != nil {
		panic(any(fmt.Errorf("marshal error: %s", err)))
	}
	err = x.storageDatabase.Put([]byte(contractAddress), blockNumber, rawValue)
	if err != nil {
		panic(any(fmt.Errorf("database error: %s", err)))
	}
}
