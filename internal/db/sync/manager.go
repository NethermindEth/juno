package sync

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/NethermindEth/juno/pkg/types"

	"github.com/NethermindEth/juno/internal/db"
)

var (
	DbError                        = errors.New("database error")
	UnmarshalError                 = errors.New("unmarshal error")
	MarshalError                   = errors.New("marshal error")
	latestBlockSavedKey            = []byte("latestBlockSaved")
	latestBlockSyncKey             = []byte("latestBlockSync")
	blockOfLatestEventProcessedKey = []byte("blockOfLatestEventProcessed")
	latestStateRoot                = []byte("latestStateRoot")
	stateDiffPrefix                = []byte("stateDiffPrefix_")
)

// Manager is a Block database manager to save and search the blocks.
type Manager struct {
	database db.Database
}

// NewManager returns a new Block manager using the given database.
func NewManager(database db.Database) *Manager {
	return &Manager{database: database}
}

// StoreLatestBlockSaved stores the latest block sync.
func (m *Manager) StoreLatestBlockSaved(latestBlockSaved int64) {
	// Marshal the latest block sync
	value, err := json.Marshal(latestBlockSaved)
	if err != nil {
		panic(any(fmt.Errorf("%w: %s", MarshalError, err)))
	}

	// Store the latest block sync
	err = m.database.Put(latestBlockSavedKey, value)
	if err != nil {
		panic(any(fmt.Errorf("%w: %s", DbError, err.Error())))
	}
}

// GetLatestBlockSaved returns the latest block sync.
func (m *Manager) GetLatestBlockSaved() int64 {
	// Query to database
	data, err := m.database.Get(latestBlockSavedKey)
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
	latestBlockSaved := new(int64)
	if err := json.Unmarshal(data, latestBlockSaved); err != nil {
		// notest
		panic(any(fmt.Errorf("%w: %s", UnmarshalError, err.Error())))
	}
	return *latestBlockSaved
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
		// notest
		if errors.Is(err, db.ErrNotFound) {
			return 0
		}
		panic(any(fmt.Errorf("%w: %s", DbError, err)))
	}
	// Unmarshal the data from database
	latestBlockSync := new(int64)
	if err := json.Unmarshal(data, latestBlockSync); err != nil {
		// notest
		panic(any(fmt.Errorf("%w: %s", UnmarshalError, err.Error())))
	}
	return *latestBlockSync
}

// StoreLatestStateRoot stores the latest state root.
func (m *Manager) StoreLatestStateRoot(stateRoot string) {
	// Store the latest state root
	err := m.database.Put(latestStateRoot, []byte(stateRoot))
	if err != nil {
		panic(any(fmt.Errorf("%w: %s", DbError, err.Error())))
	}
}

// GetLatestStateRoot returns the latest state root.
func (m *Manager) GetLatestStateRoot() string {
	// Query to database
	data, err := m.database.Get(latestStateRoot)
	if err != nil {
		// notest
		if errors.Is(err, db.ErrNotFound) {
			return ""
		}
		panic(any(fmt.Errorf("%w: %s", DbError, err)))
	}
	// Unmarshal the data from database
	return string(data)
}

// StoreBlockOfProcessedEvent stores the block of the latest event processed,
func (m *Manager) StoreBlockOfProcessedEvent(starknetFact, l1Block int64) {
	key := []byte(fmt.Sprintf("%s%d", blockOfLatestEventProcessedKey, starknetFact))
	// Marshal the latest block sync
	value, err := json.Marshal(l1Block)
	if err != nil {
		panic(any(fmt.Errorf("%w: %s", MarshalError, err)))
	}

	// Store the latest block sync
	err = m.database.Put(key, value)
	if err != nil {
		panic(any(fmt.Errorf("%w: %s", DbError, err.Error())))
	}
}

// GetBlockOfProcessedEvent returns the block of the latest event processed,
func (m *Manager) GetBlockOfProcessedEvent(starknetFact int64) int64 {
	// Query to database
	key := []byte(fmt.Sprintf("%s%d", blockOfLatestEventProcessedKey, starknetFact))
	data, err := m.database.Get(key)
	if err != nil {
		// notest
		if errors.Is(err, db.ErrNotFound) {
			return 0
		}
		panic(any(fmt.Errorf("%w: %s", DbError, err)))
	}
	// Unmarshal the data from database
	blockSync := new(int64)
	if err := json.Unmarshal(data, blockSync); err != nil {
		// notest
		panic(any(fmt.Errorf("%w: %s", UnmarshalError, err.Error())))
	}
	return *blockSync
}

// StoreStateDiff stores the state diff for the given block.
func (m *Manager) StoreStateDiff(stateDiff *types.StateDiff, blockHash string) {
	// Get the key we will use to store the state diff
	key := []byte(strconv.FormatInt(stateDiff.BlockNumber, 10))
	// Marshal the state diff
	value, err := json.Marshal(stateDiff)
	if err != nil {
		panic(any(fmt.Errorf("%w: %s", UnmarshalError, err.Error())))
	}

	// Store the stateDiff key using a blockHash
	key2 := append(stateDiffPrefix, []byte(blockHash)...)
	err = m.database.Put(key2, key)
	if err != nil {
		panic(any(fmt.Errorf("%w: %s", DbError, err.Error())))
	}

	// Store the state diff
	err = m.database.Put(key, value)
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
		// notest
		if errors.Is(err, db.ErrNotFound) {
			return nil
		}
		panic(any(fmt.Errorf("%w: %s", DbError, err)))
	}
	// Unmarshal the data from database
	stateDiff := new(types.StateDiff)
	if err := json.Unmarshal(data, stateDiff); err != nil {
		// notest
		panic(any(fmt.Errorf("%w: %s", UnmarshalError, err.Error())))
	}
	return stateDiff
}

func (m *Manager) GetStateDiffFromHash(blockHash string) *types.StateDiff {
	// Query to database
	key := append(stateDiffPrefix, []byte(blockHash)...)
	data, err := m.database.Get(key)
	if err != nil {
		// notest
		if errors.Is(err, db.ErrNotFound) {
			return nil
		}
		panic(any(fmt.Errorf("%w: %s", DbError, err)))
	}
	// Unmarshal the data from database
	blockNumber := new(int64)
	if err := json.Unmarshal(data, blockNumber); err != nil {
		// notest
		panic(any(fmt.Errorf("%w: %s", UnmarshalError, err.Error())))
	}
	return m.GetStateDiff(*blockNumber)
}

// Close closes the Manager.
func (m *Manager) Close() {
	m.database.Close()
}
