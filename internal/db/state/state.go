package state

import (
	"github.com/NethermindEth/juno/internal/db"
)

// Manager is a database manager, with the objective of managing
// the contract codes and contract storages databases.
type Manager struct {
	binaryCodeDatabase     db.Databaser
	codeDefinitionDatabase db.Databaser
	storageDatabase        *db.BlockSpecificDatabase
	trieDatabase           db.Databaser
}

// NewStateManager returns a new instance of Manager with the given database sources.
func NewStateManager(binaryCodeDb, codeDefinitionDb, trieDb db.Databaser, storageDatabase *db.BlockSpecificDatabase) *Manager {
	return &Manager{
		binaryCodeDatabase:     binaryCodeDb,
		codeDefinitionDatabase: codeDefinitionDb,
		storageDatabase:        storageDatabase,
		trieDatabase:           trieDb,
	}
}

func (m *Manager) Close() {
	m.binaryCodeDatabase.Close()
	m.storageDatabase.Close()
}
