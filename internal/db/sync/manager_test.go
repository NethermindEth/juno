package sync_test

import (
	"testing"

	"gotest.tools/assert"

	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/types"

	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/db/sync"
)

// Test the Latest Block Saved is stored and retrieved correctly.
func TestLatestBlockSaved(t *testing.T) {
	env, err := db.NewMDBXEnv(t.TempDir(), 2, 0)
	if err != nil {
		t.Error(err)
	}
	syncDatabase, err := db.NewMDBXDatabase(env, "SYNC")
	// Create a new database manager.
	manager := sync.NewManager(syncDatabase)

	// Store the latest block sync.
	manager.StoreLatestBlockSaved(1)

	// Check that the latest block sync is stored correctly.
	if manager.GetLatestBlockSaved() != 1 {
		t.Errorf("Latest block saved was not stored correctly.")
	}
	manager.Close()
}

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
	manager.Close()
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
	manager.Close()
}

// Test GetLatestStateRoot returns the correct state root.
func TestLatestStateRoot(t *testing.T) {
	env, err := db.NewMDBXEnv(t.TempDir(), 2, 0)
	if err != nil {
		t.Error(err)
	}
	syncDatabase, err := db.NewMDBXDatabase(env, "SYNC")
	// Create a new database manager.
	manager := sync.NewManager(syncDatabase)

	// Store the latest state root.
	manager.StoreLatestStateRoot("0x123")

	// Check that the latest state root is stored correctly.
	if manager.GetLatestStateRoot() != "0x123" {
		t.Errorf("Latest state root was not stored correctly.")
	}
	manager.Close()
}

// Test StateDiff is stored and retrieved correctly.
func TestStateDiff(t *testing.T) {
	env, err := db.NewMDBXEnv(t.TempDir(), 2, 0)
	if err != nil {
		t.Error(err)
	}
	syncDatabase, err := db.NewMDBXDatabase(env, "SYNC")
	// Create a new database manager.
	manager := sync.NewManager(syncDatabase)

	stateDiff := &types.StateUpdate{
		StorageDiff: map[felt.Felt][]types.MemoryCell{
			new(felt.Felt).SetHex("0x0000000000000000000000000000000000000000000000000000000000000002").Deref(): {
				{
					Address: new(felt.Felt).SetHex("0x0000000000000000000000000000000000000001"),
					Value:   new(felt.Felt).SetHex("0x0000000000000000000000000000000000000002"),
				},
			},
		},
		BlockHash: new(felt.Felt).SetHex("0x0123"),
		NewRoot:   new(felt.Felt).SetHex("0x0000000000000000000000000000000000000000000000000000000000000123"),
		OldRoot:   new(felt.Felt).SetHex("0x0000000000000000000000000000000000000000000000000000000000000001"),
		DeployedContracts: []types.DeployedContract{
			{
				Address: new(felt.Felt).SetHex("0x0000000000000000000000000000000000000000000000000000000000000002"),
				Hash:    new(felt.Felt).SetHex("0x0000000000000000000000000000000000000000000000000000000000000123"),
			},
		},
		DeclaredContracts: make([]*felt.Felt, 0),
	}

	// Store the state diff.
	if err := manager.StoreStateUpdate(stateDiff, stateDiff.BlockHash); err != nil {
		t.Error(err)
	}

	// Check that the state diff is stored correctly.
	retrievedStateDiff, err := manager.GetStateUpdate(stateDiff.BlockHash)
	if err != nil {
		t.Error(err)
	}

	// Check that the state diff is stored correctly.
	assert.DeepEqual(t, retrievedStateDiff, stateDiff)

	// Check that the state diff is stored correctly.
	retrievedStateDiff, err = manager.GetStateUpdate(stateDiff.BlockHash)
	if err != nil {
		t.Error(err)
	}

	// Check that the state diff is stored correctly.
	assert.DeepEqual(t, retrievedStateDiff, stateDiff)

	manager.Close()
}
