package abi

import (
	"github.com/NethermindEth/juno/internal/db"
	"google.golang.org/protobuf/proto"
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
	// Build the key from contract address
	key := []byte(contractAddress)
	// Query to database
	data, err := m.database.Get(key)
	if err != nil {
		return nil, err
	}
	// Unmarshal the data from database
	abi := new(Abi)
	if err := proto.Unmarshal(data, abi); err != nil {
		return nil, err
	}
	return abi, nil
}

// PutABI puts the ABI to the contract address.
func (m *Manager) PutABI(contractAddress string, abi *Abi) error {
	// Build the key from contract address
	key := []byte(contractAddress)
	value, err := proto.Marshal(abi)
	if err != nil {
		return err
	}
	if err := m.database.Put(key, value); err != nil {
		return err
	}
	return nil
}

// Close closes the associated database
func (m *Manager) Close() {
	m.database.Close()
}
