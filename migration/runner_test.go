package migration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/migration"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
)

// cancellableMockMigration is a migration that can block until context is cancelled
// or complete successfully. It's used to test cancellation behaviour during migration execution.
type cancellableMockMigration struct {
	beforeCalled      bool
	migrateCalled     bool
	intermediateState []byte
	receivedState     []byte
	completeChan      chan struct{} // If set, migration completes when channel is closed
}

func (m *cancellableMockMigration) Before(intermediateState []byte) error {
	m.beforeCalled = true
	m.receivedState = intermediateState
	return nil
}

func (m *cancellableMockMigration) Migrate(
	ctx context.Context,
	database db.KeyValueStore,
	network *utils.Network,
	log utils.StructuredLogger,
) ([]byte, error) {
	m.migrateCalled = true

	// If completeChan is set, wait for either completion or cancellation
	if m.completeChan != nil {
		select {
		case <-m.completeChan:
			// Migration completed successfully
			return nil, nil
		case <-ctx.Done():
			// Context was cancelled
			return m.intermediateState, nil
		}
	}

	// Default behaviour: block until context is cancelled
	<-ctx.Done()
	return m.intermediateState, nil
}

// Test helpers

// setupTestDB creates a new in-memory database for testing.
func setupTestDB(t *testing.T) db.KeyValueStore {
	t.Helper()
	testDB := memory.New()
	t.Cleanup(func() { testDB.Close() })
	return testDB
}

// createRunner creates a MigrationRunner with the given registry.
func createRunner(
	t *testing.T,
	testDB db.KeyValueStore,
	registry *migration.Registry,
) *migration.MigrationRunner {
	t.Helper()
	runner, err := migration.NewRunner(
		registry,
		testDB,
		&utils.Mainnet,
		utils.NewNopZapLogger(),
	)
	require.NoError(t, err)
	return runner
}

func TestNewRunner(t *testing.T) {
	t.Run("Success with empty database", func(t *testing.T) {
		testDB := setupTestDB(t)
		registry := migration.NewRegistry().With(&mockMigration{})

		runner := createRunner(t, testDB, registry)
		require.NotNil(t, runner)
	})

	t.Run("Success with existing metadata", func(t *testing.T) {
		testDB := setupTestDB(t)
		existingMetadata := migration.SchemaMetadata{
			CurrentVersion:    migration.SchemaVersion(0b001), // Migration 0 applied
			LastTargetVersion: migration.SchemaVersion(0b011), // Migrations 0 and 1 previously enabled
		}
		require.NoError(t, migration.WriteSchemaMetadata(testDB, existingMetadata))

		registry := migration.NewRegistry().
			With(&mockMigration{}).
			With(&mockMigration{})

		runner := createRunner(t, testDB, registry)
		require.NotNil(t, runner)
	})

	t.Run("Error on opt-out attempt", func(t *testing.T) { //nolint:dupl,lll,nolintlint // shares code with version downgrade, tests different error message, nolintlint because main config does not check lll in tests
		testDB := setupTestDB(t)
		existingMetadata := migration.SchemaMetadata{
			CurrentVersion:    migration.SchemaVersion(0b011), // Migrations 0 and 1 applied
			LastTargetVersion: migration.SchemaVersion(0b111), // Migrations 0, 1 and 2 previously enabled
		}
		require.NoError(t, migration.WriteSchemaMetadata(testDB, existingMetadata))

		// Create registry with migrations 0 and 1 (but not 2, which was previously enabled)
		registry := migration.NewRegistry().
			With(&mockMigration{}).
			With(&mockMigration{}).
			WithOptional(&mockMigration{}, false, "migration-2")

		runner, err := migration.NewRunner(
			registry,
			testDB,
			&utils.Mainnet,
			utils.NewNopZapLogger(),
		)
		require.Error(t, err)
		require.Nil(t, runner)
		require.Contains(t, err.Error(), "cannot opt out of previously enabled migrations:")
		require.Contains(t, err.Error(), "--migration-2")
	})

	t.Run("Error on version downgrade", func(t *testing.T) { //nolint:dupl,lll,nolintlint // shares code with opt-out attempt, tests different error message, nolintlint because main config does not check lll in tests
		testDB := setupTestDB(t)
		// Database has migration 3 applied, but target only has 2 migrations
		existingMetadata := migration.SchemaMetadata{
			CurrentVersion:    migration.SchemaVersion(0b111),
			LastTargetVersion: migration.SchemaVersion(0b111),
		}
		require.NoError(t, migration.WriteSchemaMetadata(testDB, existingMetadata))

		// Create registry with only migrations 0 and 1, but database has migration 2 applied
		registry := migration.NewRegistry().
			With(&mockMigration{}).
			With(&mockMigration{})

		runner, err := migration.NewRunner(
			registry,
			testDB,
			&utils.Mainnet,
			utils.NewNopZapLogger(),
		)
		require.Error(t, err)
		require.Nil(t, runner)
		require.Contains(t, err.Error(), "database is from a newer, incompatible version of Juno")
	})
}

func TestMigrationRunner_Run(t *testing.T) {
	t.Run("No pending migrations", func(t *testing.T) {
		testDB := setupTestDB(t)
		m := &mockMigration{}
		registry := migration.NewRegistry().With(m)
		targetVersion := registry.TargetVersion()

		// Set current version equal to target (no pending)
		metadata := migration.SchemaMetadata{
			CurrentVersion:    targetVersion,
			LastTargetVersion: targetVersion,
		}
		require.NoError(t, migration.WriteSchemaMetadata(testDB, metadata))

		runner := createRunner(t, testDB, registry)
		require.NoError(t, runner.Run(context.Background()))

		// Verify migration was not called
		require.False(t, m.beforeCalled)
		require.False(t, m.migrateCalled)
	})

	t.Run("Run single pending migration", func(t *testing.T) {
		testDB := setupTestDB(t)
		m := &mockMigration{}
		registry := migration.NewRegistry().With(m)

		runner := createRunner(t, testDB, registry)
		require.NoError(t, runner.Run(context.Background()))

		// Verify migration was called
		require.True(t, m.beforeCalled)
		require.True(t, m.migrateCalled)

		// Verify metadata was updated
		metadata, err := migration.GetSchemaMetadata(testDB)
		require.NoError(t, err)
		require.True(t, metadata.CurrentVersion.Has(0))
		require.Equal(t, registry.TargetVersion(), metadata.LastTargetVersion)
	})

	t.Run("Run multiple pending migrations in order", func(t *testing.T) {
		testDB := setupTestDB(t)
		m0 := &mockMigration{}
		m1 := &mockMigration{}
		registry := migration.NewRegistry().
			With(m0).
			With(m1)

		runner := createRunner(t, testDB, registry)
		require.NoError(t, runner.Run(context.Background()))

		// Verify both migrations were called
		require.True(t, m0.beforeCalled)
		require.True(t, m0.migrateCalled)
		require.True(t, m1.beforeCalled)
		require.True(t, m1.migrateCalled)

		// Verify metadata was updated
		metadata, err := migration.GetSchemaMetadata(testDB)
		require.NoError(t, err)
		require.True(t, metadata.CurrentVersion.Has(0))
		require.True(t, metadata.CurrentVersion.Has(1))
	})

	t.Run("Error when Before fails", func(t *testing.T) {
		testDB := setupTestDB(t)
		m := &mockMigration{beforeErr: errors.New("before failed")}
		registry := migration.NewRegistry().With(m)

		runner := createRunner(t, testDB, registry)
		err := runner.Run(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), "before failed")
	})

	t.Run("Error when Migrate fails", func(t *testing.T) {
		testDB := setupTestDB(t)
		m := &mockMigration{migrateErr: errors.New("migrate failed")}
		registry := migration.NewRegistry().With(m)

		runner := createRunner(t, testDB, registry)
		err := runner.Run(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), "migrate failed")

		// Verify metadata was not updated
		metadata, err := migration.GetSchemaMetadata(testDB)
		require.NoError(t, err)
		require.False(t, metadata.CurrentVersion.Has(0))
		require.Equal(t, registry.TargetVersion(), metadata.LastTargetVersion)
	})

	t.Run("Context cancellation before migration starts", func(t *testing.T) {
		testDB := setupTestDB(t)
		m := &mockMigration{migrateErr: context.Canceled}
		registry := migration.NewRegistry().With(m)

		runner := createRunner(t, testDB, registry)
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := runner.Run(ctx)
		require.Error(t, err)
		require.True(t, errors.Is(err, context.Canceled))
	})

	t.Run("Migration with intermediate state", func(t *testing.T) {
		testDB := setupTestDB(t)
		intermediateState := []byte("test state")
		m := &mockMigration{intermediateState: intermediateState}
		registry := migration.NewRegistry().With(m)

		runner := createRunner(t, testDB, registry)
		require.NoError(t, runner.Run(context.Background()))

		// Verify intermediate state was saved
		savedState, err := migration.GetIntermediateState(testDB, 0)
		require.NoError(t, err)
		require.Equal(t, intermediateState, savedState)

		// Verify metadata
		metadata, err := migration.GetSchemaMetadata(testDB)
		require.NoError(t, err)
		require.False(t, metadata.CurrentVersion.Has(0))
		require.Equal(t, registry.TargetVersion(), metadata.LastTargetVersion)
	})

	t.Run("Resume migration from intermediate state", func(t *testing.T) {
		testDB := setupTestDB(t)
		intermediateState := []byte("previous state")
		require.NoError(t, migration.WriteIntermediateState(testDB, 0, intermediateState))

		// Save metadata showing migration 0 is in progress
		metadata := migration.SchemaMetadata{
			CurrentVersion:    migration.SchemaVersion(0),
			LastTargetVersion: migration.SchemaVersion(0b001),
		}
		require.NoError(t, migration.WriteSchemaMetadata(testDB, metadata))

		m := &mockMigration{}
		registry := migration.NewRegistry().With(m)

		runner := createRunner(t, testDB, registry)
		require.NoError(t, runner.Run(context.Background()))

		// Verify Before was called with intermediate state
		require.True(t, m.beforeCalled)
		require.Equal(t, intermediateState, m.receivedState)
	})

	t.Run("Complete migration clears intermediate state", func(t *testing.T) {
		testDB := setupTestDB(t)
		intermediateState := []byte("test state")
		require.NoError(t, migration.WriteIntermediateState(testDB, 0, intermediateState))

		// Migration completes (returns nil intermediate state)
		m := &mockMigration{intermediateState: nil}
		registry := migration.NewRegistry().With(m)

		runner := createRunner(t, testDB, registry)
		require.NoError(t, runner.Run(context.Background()))

		// Verify intermediate state was cleared
		_, err := migration.GetIntermediateState(testDB, 0)
		require.ErrorIs(t, err, db.ErrKeyNotFound)

		// Verify migration was marked as complete
		metadata, err := migration.GetSchemaMetadata(testDB)
		require.NoError(t, err)
		require.True(t, metadata.CurrentVersion.Has(0))
	})

	t.Run("Intermediate state preservation with optional migration", func(t *testing.T) {
		// Scenario: Migration 0 is optional (initially disabled, then enabled)
		// Migration 1 is mandatory (higher index)
		// Test that intermediate state of higher index migration is preserved
		// when optional migration is enabled/disabled between runs

		testDB := setupTestDB(t)

		t.Run("Step 1: Optional migration disabled, mandatory migration cancelled", func(t *testing.T) {
			registry := migration.NewRegistry()
			m0 := &mockMigration{}
			m1 := &cancellableMockMigration{
				intermediateState: []byte("migration1-state"),
			}

			registry.WithOptional(m0, false, "migration-0") // Not enabled
			registry.With(m1)                               // Mandatory

			require.False(t, registry.TargetVersion().Has(0))
			require.True(t, registry.TargetVersion().Has(1))

			runner := createRunner(t, testDB, registry)
			ctx, cancel := context.WithDeadline(
				context.Background(),
				time.Now().Add(10*time.Millisecond),
			)
			defer cancel()

			err := runner.Run(ctx)
			require.ErrorIs(t, err, ctx.Err())

			// Verify migration 1's intermediate state was saved
			savedState1, err := migration.GetIntermediateState(testDB, 1)
			require.NoError(t, err)
			require.Equal(t, []byte("migration1-state"), savedState1)

			// Verify migration 0 was not run (not in target)
			require.False(t, m0.beforeCalled)
			require.False(t, m0.migrateCalled)
		})

		t.Run("Step 2: Optional migration enabled, runs and cancelled", func(t *testing.T) {
			registry := migration.NewRegistry()
			m0 := &cancellableMockMigration{
				intermediateState: []byte("migration0-state"),
			}
			m1 := &mockMigration{} // Will resume from intermediate state

			registry.WithOptional(m0, true, "migration-0") // NOW enabled
			registry.With(m1)                              // Mandatory

			require.True(t, registry.TargetVersion().Has(0))
			require.True(t, registry.TargetVersion().Has(1))

			runner := createRunner(t, testDB, registry)
			ctx, cancel := context.WithDeadline(
				context.Background(),
				time.Now().Add(20*time.Millisecond),
			)
			defer cancel()

			err := runner.Run(ctx)
			require.ErrorIs(t, err, ctx.Err())

			// Verify migration 0's intermediate state was saved
			savedState0, err := migration.GetIntermediateState(testDB, 0)
			require.NoError(t, err)
			require.Equal(t, []byte("migration0-state"), savedState0)

			// Verify migration 1's intermediate state is still preserved
			savedState1, err := migration.GetIntermediateState(testDB, 1)
			require.NoError(t, err)
			require.Equal(t, []byte("migration1-state"), savedState1)

			// Verify migration 1 was not run yet (migration 0 ran first and cancelled)
			require.False(t, m1.beforeCalled)
		})

		t.Run("Step 3: Both migrations resume and complete", func(t *testing.T) {
			registry := migration.NewRegistry()
			m0 := &mockMigration{} // Will resume from intermediate state
			m1 := &mockMigration{} // Will resume from intermediate state

			registry.WithOptional(m0, true, "migration-0")
			registry.With(m1)

			runner := createRunner(t, testDB, registry)
			require.NoError(t, runner.Run(context.Background()))

			// Verify migration 0 resumed from its intermediate state
			require.True(t, m0.beforeCalled)
			require.Equal(t, []byte("migration0-state"), m0.receivedState)

			// Verify migration 1 resumed from its intermediate state
			require.True(t, m1.beforeCalled)
			require.Equal(t, []byte("migration1-state"), m1.receivedState)

			// Verify final metadata
			metadata, err := migration.GetSchemaMetadata(testDB)
			require.NoError(t, err)
			require.True(t, metadata.CurrentVersion.Has(0))
			require.True(t, metadata.CurrentVersion.Has(1))

			// Verify intermediate states are cleared after completion
			_, err = migration.GetIntermediateState(testDB, 0)
			require.ErrorIs(t, err, db.ErrKeyNotFound)

			_, err = migration.GetIntermediateState(testDB, 1)
			require.ErrorIs(t, err, db.ErrKeyNotFound)
		})
	})
}
