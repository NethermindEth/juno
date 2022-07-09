package abi

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/internal/db"
	"google.golang.org/protobuf/proto"
)

var (
	DbError        = errors.New("database error")
	UnmarshalError = errors.New("unmarshal error")
	MarshalError   = errors.New("marshal error")
)

// Manager is a database to store and get the contracts ABI.
type Manager struct {
	database db.Database
}

// NewABIManager creates a new Manager instance.
func NewABIManager(database db.Database) *Manager {
	return &Manager{database}
}

// GetABI gets the ABI associated with the contract address. If the ABI does
// not exist, then returns nil without error.
func (m *Manager) GetABI(contractAddress string) (*Abi, error) {
	key := []byte(contractAddress)   // Build the key from contract address
	data, err := m.database.Get(key) // Query to database

	if err != nil {
		if db.ErrNotFound == err {
			//TODO: talk with team about it.
			//Higher level should be the one handling what to do with type of error
			//we now use %w, error.is can be used
			//return nil, errors.New("GetABI: " + db.ErrNotFound.Error())
			return nil, nil
		}
		return nil, fmt.Errorf("GetABI: failed get operation: %w", err)
	}
	if data == nil {
		// notest
		return nil, fmt.Errorf("GetABI: data from key %s from get operation is null", key)
	}
	// Unmarshal the data from database
	abi := new(Abi)
	if err := proto.Unmarshal(data, abi); err != nil {
		// notest
		return nil, fmt.Errorf("GetABI: %s: %w", UnmarshalError, err)
	}

	return abi, nil
}

// PutABI puts the ABI to the contract address.
func (m *Manager) PutABI(contractAddress string, abi *Abi) error {
	// Build the key from contract address
	key := []byte(contractAddress)
	value, err := proto.Marshal(abi)
	if err != nil {
		// notest
		return fmt.Errorf("PutABI: %s: %w", MarshalError, err)
	}
	err = m.database.Put(key, value)
	if err != nil {
		// notest
		return fmt.Errorf("PutABI: failed put operation: %w", err)
	}
	return nil
}

// Close closes the associated database
func (m *Manager) Close() {
	m.database.Close()
}
