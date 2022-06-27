package sync

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/NethermindEth/juno/internal/db"
)

var (
	DbError               = errors.New("database error")
	UnmarshalError        = errors.New("unmarshal error")
	MarshalError          = errors.New("marshal error")
	latestBlockSyncKey    = []byte("latestBlockSync")
	latestStateDiffSynced = []byte("latestStateDiffSynced")
)

// Manager is a Block database manager to save and search the blocks.
type Manager struct {
	database db.Database
}

// NewSyncManager returns a new Block manager using the given database.
func NewSyncManager(database db.Database) *Manager {
	return &Manager{database: database}
}

// StoreLatestBlockSync stores the latest block sync.
func (m *Manager) StoreLatestBlockSync(latestBlockSync int64) {

	// Marshal the latest block sync
	value, err := json.Marshal(latestBlockSync)
	if err != nil {
		panic(any(fmt.Errorf("%w: %s", MarshalError, err)))
	}

	// Store the latest block sync
	err = m.database.Put(latestBlockSyncKey, value)
	if err != nil {
		panic(any(fmt.Errorf("%w: %s", DbError, err.Error())))
	}
}

// GetLatestBlockSync returns the latest block sync.
func (m *Manager) GetLatestBlockSync() int64 {
	// Query to database
	data, err := m.database.Get(latestBlockSyncKey)
	if err != nil {
		if db.ErrNotFound == err {
			return 0
		}
		// notest
		panic(any(fmt.Errorf("%w: %s", DbError, err)))
	}
	if data == nil {
		// notest
		return 0
	}
	// Unmarshal the data from database
	latestBlockSync := new(int64)
	if err := json.Unmarshal(data, latestBlockSync); err != nil {
		// notest
		panic(any(fmt.Errorf("%w: %s", UnmarshalError, err.Error())))
	}
	return *latestBlockSync
}

// Close closes the Manager.
func (m *Manager) Close() {
	m.database.Close()
}
