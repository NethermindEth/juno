package sync_test

import (
	"testing"

	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/db/sync"
)

// Test that the latest block sync is stored and retrieved correctly.
func TestLatestBlockSync(t *testing.T) {
	env, err := db.NewMDBXEnv(t.TempDir(), 2, 0)
	if err != nil {
		t.Error(err)
	}
	syncDatabase, err := db.NewMDBXDatabase(env, "SYNC")
	// Create a new database manager.
	manager := sync.NewManager(syncDatabase)

	// Store the latest block sync.
	manager.StoreLatestBlockSync(1)

	// Check that the latest block sync is stored correctly.
	if manager.GetLatestBlockSync() != 1 {
		t.Errorf("Latest block sync was not stored correctly.")
	}
}

// Test that the Stored block of the latest event processed is stored and retrieved correctly.
func TestLatestEventProcessed(t *testing.T) {
	env, err := db.NewMDBXEnv(t.TempDir(), 2, 0)
	if err != nil {
		t.Error(err)
	}
	syncDatabase, err := db.NewMDBXDatabase(env, "SYNC")
	// Create a new database manager.
	manager := sync.NewManager(syncDatabase)

	// Store the latest block sync.
	manager.StoreBlockOfProcessedEvent(1, 2)

	// Check that the latest block sync is stored correctly.
	if manager.GetBlockOfProcessedEvent(1) != 2 {
		t.Errorf("Latest event processed was not stored correctly.")
	}
}
