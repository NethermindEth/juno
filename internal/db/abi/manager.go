package abi

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/NethermindEth/juno/internal/db"
	"math/big"
)

var (
	DbError                = errors.New("database error")
	UnmarshalError         = errors.New("unmarshal error")
	MarshalError           = errors.New("marshal error")
	InvalidContractAddress = errors.New("invalid contract address")
)

// Manager is a database to store and get the contracts ABI.
type Manager struct {
	database db.Databaser
}

// NewABIManager creates a new Manager instance.
func NewABIManager(database db.Databaser) *Manager {
	return &Manager{database}
}

// GetABI gets the ABI associated with the contract address. The contract address
// must be a hexadecimal string without the 0x prefix, if the contract address encoding
// is invalid then an InvalidContractAddress error is returned. If the ABI does
// not exist, then returns nil without error.
func (m *Manager) GetABI(contractAddress string) *Abi {
	// Build the key from contract address
	key := buildKey(contractAddress)
	// Query to database
	data, err := m.database.Get(key)
	if err != nil {
		// notest
		panic(any(fmt.Errorf("%w: %s", DbError, err)))
	}
	if data == nil {
		return nil
	}
	// Unmarshal the data from database
	abi := new(Abi)
	if err := json.Unmarshal(data, abi); err != nil {
		// notest
		panic(any(fmt.Errorf("%w: %s", UnmarshalError, err.Error())))
	}
	return abi
}

// PutABI puts the ABI to the contract address. The contract address must be a
// hexadecimal string without the 0x prefix, if the contract address is invalid
// then an InvalidContractAddress error is returned.
func (m *Manager) PutABI(contractAddress string, abi *Abi) {
	// Build the key from contract address
	key := buildKey(contractAddress)
	value, err := json.Marshal(abi)
	if err != nil {
		// notest
		panic(any(fmt.Errorf("%w: %s", MarshalError, err.Error())))
	}
	err = m.database.Put(key, value)
	if err != nil {
		// notest
		panic(any(fmt.Errorf("%w: %s", DbError, err.Error())))
	}
}

func buildKey(contractAddress string) []byte {
	address, ok := new(big.Int).SetString(contractAddress, 16)
	if !ok {
		panic(fmt.Errorf("%w: %s", InvalidContractAddress, contractAddress))
	}
	return address.Bytes()
}
