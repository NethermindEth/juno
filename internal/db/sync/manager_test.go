package sync_test

import (
	"reflect"
	"testing"

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

	stateDiff := &types.StateDiff{
		StorageDiff: map[string][]types.MemoryCell{
			"0x0000000000000000000000000000000000000000000000000000000000000002": {
				{
					Address: new(felt.Felt).SetHex("0x0000000000000000000000000000000000000001"),
					Value:   new(felt.Felt).SetHex("0x0000000000000000000000000000000000000002"),
				},
			},
		},
		BlockNumber: 5,
		NewRoot:     new(felt.Felt).SetHex("0x0000000000000000000000000000000000000000000000000000000000000123"),
		OldRoot:     new(felt.Felt).SetHex("0x0000000000000000000000000000000000000000000000000000000000000001"),
		DeployedContracts: []types.DeployedContract{
			{
				Address: new(felt.Felt).SetHex("0x0000000000000000000000000000000000000000000000000000000000000002"),
				Hash:    new(felt.Felt).SetHex("0x0000000000000000000000000000000000000000000000000000000000000123"),
				ConstructorCallData: []*felt.Felt{
					new(felt.Felt).SetHex("0x0000000000000000000000000000000000000000000000000000000000000003"),
					new(felt.Felt).SetHex("0x0000000000000000000000000000000000000000000000000000000000000004"),
				},
			},
		},
	}

	// Store the state diff.
	manager.StoreStateDiff(stateDiff, "0x123")

	// Check that the state diff is stored correctly.
	retrievedStateDiff := manager.GetStateDiff(1)

	// Check that the state diff is stored correctly.
	if reflect.DeepEqual(retrievedStateDiff, stateDiff) {
		t.Errorf("State diff was not stored correctly.")
	}

	retrievedStateDiffByHash := manager.GetStateDiffFromHash("0x123")
	if !reflect.DeepEqual(retrievedStateDiffByHash, stateDiff) {
		t.Errorf("State diff was not stored correctly.")
	}
	manager.Close()
}
