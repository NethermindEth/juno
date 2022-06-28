package state

import (
	"github.com/NethermindEth/juno/internal/db"
)

// Manager is a database manager, with the objective of managing
// the contract codes and contract storages databases.
type Manager struct {
	binaryCodeDatabase     db.Databaser
	codeDefinitionDatabase db.Databaser
	trieDatabase           db.Databaser
    contractStateDatabase  db.Databaser
}

// NewStateManager returns a new instance of Manager with the given database sources.
func NewStateManager(binaryCodeDb, codeDefinitionDb, trieDb, contractStateDb db.Databaser) *Manager {
	return &Manager{
		binaryCodeDatabase:     binaryCodeDb,
		codeDefinitionDatabase: codeDefinitionDb,
		trieDatabase:           trieDb,
        contractStateDatabase:  contractStateDb,
	}
}

func (m *Manager) Close() {
	m.binaryCodeDatabase.Close()
    m.codeDefinitionDatabase.Close()
    m.trieDatabase.Close()
    m.contractStateDatabase.Close()
}
