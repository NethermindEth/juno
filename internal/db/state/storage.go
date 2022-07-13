package state

import (
	"fmt"
	"github.com/NethermindEth/juno/internal/db"

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
// contract storage does not exist then returns error.
func (x *Manager) GetStorage(contractAddress string, blockNumber uint64) (*Storage, error) {
	rawData, err := x.storageDatabase.Get([]byte(contractAddress), blockNumber)
	if err != nil {
		return nil, fmt.Errorf("GetStorage: failed get operation: %w", err)
	}
	// Check not found
	if rawData == nil {
		// notest
		return nil, fmt.Errorf("GetStorage: %w", db.ErrNotFound)
	}
	value := new(Storage)
	err = proto.Unmarshal(rawData, value)
	if err != nil {
		return nil, fmt.Errorf("GetStorage: %w: %s", db.ErrUnmarshal, err)
	}
	return value, nil
}

// PutStorage saves a new version of the contract storage at the given block
// number. Returns error if error occurs.
func (x *Manager) PutStorage(contractAddress string, blockNumber uint64, storage *Storage) error {
	rawValue, err := proto.Marshal(storage)
	if err != nil {
		return fmt.Errorf("PutStorage: %s: %w", db.ErrMarshal, err)
	}
	err = x.storageDatabase.Put([]byte(contractAddress), blockNumber, rawValue)
	if err != nil {
		return fmt.Errorf("PutStorage: failed put operation: %w", err)
	}
	return nil
}
