package state

import (
	"github.com/NethermindEth/juno/internal/db"
)

// Manager is a database manager, with the objective of managing
// the contract codes and contract storages databases.
type Manager struct {
	stateDatabase db.Database
}

// NewManager returns a new instance of Manager with the given database sources.
func NewManager(stateDatabase db.Database) *Manager {
	return &Manager{stateDatabase}
}

func (m *Manager) Close() {
	m.stateDatabase.Close()
}
