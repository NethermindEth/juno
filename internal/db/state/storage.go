package state

import (
	"fmt"

	"google.golang.org/protobuf/proto"
)

func (s *Storage) Update(other *Storage) {
	// notest
	if s.Storage == nil {
		s.Storage = make(map[string]string)
	}
	for key, value := range other.Storage {
		s.Storage[key] = value
	}
}

// GetStorage returns the ContractStorage state of the given contract address
// and block number. If no exists a version for exactly the given block number,
// then returns the newest version lower than the given block number. If the
// contract storage does not exist then returns nil.
func (x *Manager) GetStorage(contractAddress string, blockNumber uint64) (*Storage, error) {
	rawData, err := x.storageDatabase.Get([]byte(contractAddress), blockNumber)
	if err != nil {
		return nil, fmt.Errorf("GetStorage: failed get database call: %w", err)
	}
	// Check not found
	if rawData == nil {
		// notest
		return nil, nil
	}
	value := new(Storage)
	err = proto.Unmarshal(rawData, value)
	if err != nil {
		return nil, fmt.Errorf("GetStorage: %w: %s", ErrUnmarshalErr, err)
	}
	return value, nil
}

// PutStorage saves a new version of the contract storage at the given block
// number.
func (x *Manager) PutStorage(contractAddress string, blockNumber uint64, storage *Storage) error {
	rawValue, err := proto.Marshal(storage)
	if err != nil {
		return fmt.Errorf("PutStorage: %w: %s", ErrMarshalErr, err)
	}
	err = x.storageDatabase.Put([]byte(contractAddress), blockNumber, rawValue)
	if err != nil {
		return fmt.Errorf("PutStorage: failed put database call: %w", err)
	}
	return nil
}
