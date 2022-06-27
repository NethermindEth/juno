package sync

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/pkg/starknet/types"
	"strconv"
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

// StoreStateDiff stores the state diff for the given block.
func (m *Manager) StoreStateDiff(stateDiff *types.StateDiff) {

	// Get the key we will use to store the state diff
	key := []byte(strconv.FormatInt(stateDiff.BlockNumber, 10))
	// Marshal the state diff
	value, err := json.Marshal(stateDiff)
	if err != nil {
		panic(any(fmt.Errorf("%w: %s", UnmarshalError, err.Error())))
	}

	// Store the state diff
	err = m.database.Put(key, value)
	if err != nil {
		panic(any(fmt.Errorf("%w: %s", DbError, err.Error())))
	}

	// Store the latest state diff synced
	err = m.database.Put(latestStateDiffSynced, []byte(strconv.FormatInt(stateDiff.BlockNumber, 10)))
	if err != nil {
		panic(any(fmt.Errorf("%w: %s", DbError, err.Error())))
	}
}

// GetStateDiff returns the state diff for the given block.
func (m *Manager) GetStateDiff(blockNumber int64) *types.StateDiff {

	// Get the key we will use to fetch the state diff
	key := []byte(strconv.FormatInt(blockNumber, 10))
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
	stateDiff := new(types.StateDiff)
	if err := json.Unmarshal(data, stateDiff); err != nil {
		// notest
		panic(any(fmt.Errorf("%w: %s", UnmarshalError, err.Error())))
	}
	return stateDiff
}

// GetLatestStateDiffSynced returns the latest StateDiff synced
func (m *Manager) GetLatestStateDiffSynced() int64 {
	// Query to database
	data, err := m.database.Get(latestStateDiffSynced)
	if err != nil {
		if db.ErrNotFound == err {
			return -1
		}
		// notest
		panic(any(fmt.Errorf("%w: %s", DbError, err)))
	}
	if data == nil {
		// notest
		return 0
	}
	// Unmarshal the data from database
	value := new(int64)
	if err := json.Unmarshal(data, value); err != nil {
		// notest
		panic(any(fmt.Errorf("%w: %s", UnmarshalError, err.Error())))
	}
	return *value
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
