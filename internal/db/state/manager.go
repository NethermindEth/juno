package state

import (
	"github.com/NethermindEth/juno/internal/db"
)

// Manager is a database manager, with the objective of managing
// the contract codes and contract storages databases.
type Manager struct {
	stateDatabase          db.Database
	binaryCodeDatabase     db.Database
	codeDefinitionDatabase db.Database
}

// NewStateManager returns a new instance of Manager with the given database sources.
func NewStateManager(stateDatabase, binaryCodeDatabase, codeDefinitionDatabase db.Database) *Manager {
	return &Manager{stateDatabase, binaryCodeDatabase, codeDefinitionDatabase}
}

func (m *Manager) Close() {
	m.stateDatabase.Close()
	m.binaryCodeDatabase.Close()
	m.codeDefinitionDatabase.Close()
}
