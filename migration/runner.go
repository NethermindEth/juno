package migration

import (
	"context"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils"
	"go.uber.org/zap"
)

// Migration represents a migration that can be applied to the database.
//
// Execution Flow:
//  1. Before(intermediateState) is called to restore state before each execution attempt
//  2. Migrate() is called to perform the migration work
//  3. Based on Migrate() return values:
//     - (nil, nil): Migration complete, marked as applied
//     - (state, nil): Migration in progress, state saved for resumption
//     - (state, error): Error occurred; if context.Canceled, state is saved if non-nil
//
// Intermediate State:
//   - Before() receives intermediateState loaded from the database (nil on first run)
//   - Migrate() returns intermediateState to allow resumable migrations
//   - If Migrate() returns non-nil state, it's saved and the migration will resume on next run
//   - If Migrate() returns nil state, the migration is marked complete and state is cleared
//   - On cancellation, non-nil state is saved; nil state clears any existing state
//
// This two-phase design separates state restoration from migration work, allowing
// Migrate() to focus solely on doing the work while Before() handles state management.
type Migration interface {
	// Before restores internal state from a previous migration attempt.
	// Called before each execution of Migrate(), including first-time runs.
	// intermediateState is nil for first-time execution, non-nil when resuming.
	Before(intermediateState []byte) error

	// Migrate executes the migration logic.
	// Returns (intermediateState, error):
	//   - (nil, nil): Migration complete
	//   - (state, nil): Migration in progress, will resume with this state
	//   - (state, error): Error occurred; state is saved if error is context.Canceled
	//   - (nil, error): Error occurred; any existing state is cleared
	Migrate(
		ctx context.Context,
		database db.KeyValueStore,
		network *utils.Network,
		log utils.StructuredLogger,
	) ([]byte, error)
}

type MigrationRunner struct {
	entries       []Migration
	targetVersion SchemaVersion
	metadata      SchemaMetadata
	database      db.KeyValueStore
	network       *utils.Network
	log           utils.StructuredLogger
}

// NewRunner creates a migration runner that executes pending migrations.
//
// Parameters:
//   - entries: All registered migrations
//   - targetVersion: End state to reach after running all migrations
//   - database: Database store
//   - network: Network configuration
//   - log: Logger for migration progress
func NewRunner(
	entries []Migration,
	targetVersion SchemaVersion,
	database db.KeyValueStore,
	network *utils.Network,
	log utils.StructuredLogger,
) (*MigrationRunner, error) {
	// Validate that entries length matches the highest migration index + 1
	// If targetVersion is empty (no migrations), HighestBit() returns -1
	expectedLen := targetVersion.HighestBit() + 1
	if len(entries) != expectedLen {
		return nil, fmt.Errorf("entries length mismatch: %d != %d",
			len(entries),
			expectedLen,
		)
	}
	metadata, err := GetSchemaMetadata(database)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, err
	}

	if errors.Is(err, db.ErrKeyNotFound) {
		metadata = SchemaMetadata{
			CurrentVersion:    0,
			LastTargetVersion: 0,
		}
	}

	// Validate: check if database was migrated with a newer version of Juno (version downgrade)
	if err := validateNoVersionDowngrade(metadata.CurrentVersion, entries); err != nil {
		return nil, err
	}

	// Validate: check if any previously opted-in migrations are now missing (opt-out attempt)
	if err := validateNoOptOut(targetVersion, metadata.LastTargetVersion); err != nil {
		return nil, err
	}

	return &MigrationRunner{
		entries:       entries,
		targetVersion: targetVersion,
		metadata:      metadata,
		database:      database,
		network:       network,
		log:           log,
	}, nil
}

// Run executes all pending migrations in order.
func (mr *MigrationRunner) Run(ctx context.Context) error {
	// Save target version at the beginning to record intent, even if migration fails midway
	// This ensures future runs can prevent opt-out of currently enabled migrations
	mr.metadata.LastTargetVersion = mr.targetVersion
	if err := WriteSchemaMetadata(mr.database, mr.metadata); err != nil {
		return err
	}

	// Derive pending from target and current: migrations in target but not yet applied
	pending := mr.targetVersion.Difference(mr.metadata.CurrentVersion)
	if pending == 0 {
		return nil // No migrations to run
	}

	totalInTarget := mr.targetVersion.HighestBit() + 1
	// Execute each pending migration
	for migrationIndex := range pending.Iter() {
		if err := ctx.Err(); err != nil {
			return err
		}

		mr.log.Info("Applying migration",
			zap.String("progress", fmt.Sprintf("%d/%d", migrationIndex+1, totalInTarget)),
		)

		err := mr.runMigration(ctx, migrationIndex)
		if err != nil {
			return err
		}

		mr.log.Info("Migration applied",
			zap.String("progress", fmt.Sprintf("%d/%d", migrationIndex+1, totalInTarget)),
		)
	}

	return ctx.Err()
}

func (mr *MigrationRunner) runMigration(ctx context.Context, migrationIndex uint8) error {
	migration := mr.entries[migrationIndex]
	intermediateState, err := GetIntermediateState(mr.database, migrationIndex)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return err
	}

	if errors.Is(err, db.ErrKeyNotFound) {
		intermediateState = nil
	}

	if err := migration.Before(intermediateState); err != nil {
		return err
	}

	intermediateState, err = migration.Migrate(ctx, mr.database, mr.network, mr.log)

	if err != nil && !errors.Is(err, ctx.Err()) {
		return err
	}

	if intermediateState != nil {
		// Migration produced intermediate state - save it for resumption
		if err := WriteIntermediateState(mr.database, migrationIndex, intermediateState); err != nil {
			return err
		}
		return ctx.Err()
	}

	mr.metadata.CurrentVersion.Set(migrationIndex)
	txn := mr.database.NewBatch()
	if err := WriteSchemaMetadata(txn, mr.metadata); err != nil {
		return err
	}

	if err := DeleteIntermediateState(txn, migrationIndex); err != nil {
		return err
	}

	return txn.Write()
}

// validateNoVersionDowngrade checks if the database was migrated with a newer version of Juno
// that has migrations not present in the current codebase.
// Returns an error if current has migrations beyond what's available in entries, nil otherwise.
func validateNoVersionDowngrade(current SchemaVersion, entries []Migration) error {
	maxAvailableIndex := len(entries) - 1

	// Check if current has any migrations beyond what's available in the codebase
	if current.HighestBit() > maxAvailableIndex {
		return errors.New(
			"database is from a newer, incompatible version of Juno; upgrade to use this database",
		)
	}

	return nil
}

// validateNoOptOut checks if any previously opted-in migrations (in lastTargetVersion)
// are missing from the target version (opt-out attempt).
// Returns an error if opt-out attempts are detected, nil otherwise.
func validateNoOptOut(target, lastTargetVersion SchemaVersion) error {
	optOutAttempts := lastTargetVersion.Difference(target)
	if optOutAttempts != 0 {
		return fmt.Errorf("cannot opt out of previously enabled migrations: %s", optOutAttempts.String())
	}
	return nil
}
