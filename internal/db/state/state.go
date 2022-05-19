package state

import (
	"errors"
	db2 "github.com/NethermindEth/juno/internal/db"
)

var (
	DbError                = errors.New("database error")
	InvalidContractAddress = errors.New("invalid contract address")
	UnmarshalError         = errors.New("unmarshal error")
	MarshalError           = errors.New("marshal error")
)

// Manager is a database manager, with the objective of managing
// the contract codes and contract storages databases.
type Manager struct {
	codeDatabase    db2.Databaser
	storageDatabase db2.BlockSpecificDatabase
}

// NewStateManager returns a new instance of Manager with the given database sources.
func NewStateManager(codeDatabase db2.Databaser, storageDatabase db2.BlockSpecificDatabase) *Manager {
	return &Manager{codeDatabase, storageDatabase}
}
