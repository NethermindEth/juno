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
	database db.Databaser
}

// NewABIManager creates a new Manager instance.
func NewABIManager(database db.Databaser) *Manager {
	return &Manager{database}
}

// GetABI gets the ABI associated with the contract address. If the ABI does
// not exist, then returns nil without error.
func (m *Manager) GetABI(contractAddress string) *Abi {
	// Build the key from contract address
	key := []byte(contractAddress)
	// Query to database
	data, err := m.database.Get(key)
	if err != nil {
		if db.ErrNotFound == err {
			return nil
		}
		// notest
		panic(any(fmt.Errorf("%w: %s", DbError, err)))
	}
	if data == nil {
		// notest
		return nil
	}
	// Unmarshal the data from database
	abi := new(Abi)
	if err := proto.Unmarshal(data, abi); err != nil {
		// notest
		panic(any(fmt.Errorf("%w: %s", UnmarshalError, err.Error())))
	}
	return abi
}

// PutABI puts the ABI to the contract address.
func (m *Manager) PutABI(contractAddress string, abi *Abi) {
	// Build the key from contract address
	key := []byte(contractAddress)
	value, err := proto.Marshal(abi)
	if err != nil {
		// notest
		panic(any(fmt.Errorf("%w: %s", MarshalError, err)))
	}
	err = m.database.Put(key, value)
	if err != nil {
		// notest
		panic(any(fmt.Errorf("%w: %s", DbError, err.Error())))
	}
}

// Close closes the associated database
func (m *Manager) Close() {
	m.database.Close()
}
