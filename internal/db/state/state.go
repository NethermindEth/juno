package state

import (
	"github.com/NethermindEth/juno/internal/db"
)

// Manager is a database manager, with the objective of managing
// the contract codes and contract storages databases.
type Manager struct {
	codeDatabase    db.Database
	storageDatabase *db.BlockSpecificDatabase
}

// NewManager returns a new instance of Manager with the given database sources.
func NewManager(codeDatabase db.Database, storageDatabase *db.BlockSpecificDatabase) *Manager {
	return &Manager{codeDatabase, storageDatabase}
}

func (m *Manager) Close() {
	m.codeDatabase.Close()
	m.storageDatabase.Close()
}
