package migration

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"runtime"
	"sync"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils"
	"github.com/fxamacker/cbor/v2"
	"github.com/sourcegraph/conc/pool"
)

type schemaMetadata struct {
	Version           uint64
	IntermediateState []byte
}

type Migration interface {
	Before(intermediateState []byte) error
	// Migration should return intermediate state whenever it requests new txn or detects cancelled ctx.
	Migrate(context.Context, db.Transaction, *utils.Network, utils.SimpleLogger) ([]byte, error)
}

type MigrationFunc func(db.Transaction, *utils.Network) error

// Migrate returns f(txn).
func (f MigrationFunc) Migrate(_ context.Context, txn db.Transaction, network *utils.Network, _ utils.SimpleLogger) ([]byte, error) {
	return nil, f(txn, network)
}

// Before is a no-op.
func (f MigrationFunc) Before(_ []byte) error { return nil }

// todo comment
const startMigrationVersion = 17

// defaultMigrations contains a set of migrations that can be applied to a database.
// After making breaking changes to the DB layout, add new migrations to this list.
var defaultMigrations = []Migration{
	MigrationFunc(calculateL1MsgHashes),
	MigrationFunc(removePendingBlock),
}

var ErrCallWithNewTransaction = errors.New("call with new transaction")

func MigrateIfNeeded(ctx context.Context, targetDB db.DB, network *utils.Network, log utils.SimpleLogger) error {
	return migrateIfNeeded(ctx, targetDB, network, log, defaultMigrations)
}

func migrateIfNeeded(ctx context.Context, targetDB db.DB, network *utils.Network, log utils.SimpleLogger, migrations []Migration) error {
	/*
		Schema metadata of the targetDB determines which set of migrations need to be applied to the database.
		After a migration is successfully executed, which may update the database, the schema version is incremented
		by 1 by this loop.

		For example;

		Assume an empty DB, which has a schema version of 0. So we start applying the migrations from the
		very first one in the list and increment the schema version as we go. After the loop exists we
		end up with a database which's schema version is len(migrations).

		After running that Juno version for a while, if the user updates its Juno version which adds new
		migrations to the list, MigrateIfNeeded will skip the already applied migrations and only apply the
		new ones. It will be able to do this since the schema version it reads from the database will be
		non-zero and that is what we use to initialise the version loop variable.
	*/
	metadata, err := SchemaMetadata(targetDB)
	if err != nil {
		return err
	}

	// in case of empty db we update metadata like all previous migrations were applied
	if metadata.Version == 0 {
		metadata.Version = startMigrationVersion
	}

	currentVersion := startMigrationVersion + uint64(len(migrations))
	if metadata.Version > currentVersion {
		return errors.New("db is from a newer, incompatible version of Juno; upgrade to use this database")
	}

	for version := metadata.Version; version < currentVersion; version++ {
		if err = ctx.Err(); err != nil {
			return err
		}
		log.Infow("Applying database migration", "stage", fmt.Sprintf("%d/%d", version+1, currentVersion))
		i := version - startMigrationVersion
		migration := migrations[i]
		if err = migration.Before(metadata.IntermediateState); err != nil {
			return err
		}
		for {
			callWithNewTransaction := false
			if dbErr := targetDB.Update(func(txn db.Transaction) error {
				metadata.IntermediateState, err = migration.Migrate(ctx, txn, network, log)
				switch {
				case err == nil || errors.Is(err, ctx.Err()):
					if metadata.IntermediateState == nil {
						metadata.Version++
					}
					return updateSchemaMetadata(txn, metadata)
				case errors.Is(err, ErrCallWithNewTransaction):
					callWithNewTransaction = true
					return nil
				default:
					return err
				}
			}); dbErr != nil {
				return dbErr
			} else if !callWithNewTransaction {
				break
			}
		}
	}

	return nil
}

// SchemaMetadata retrieves metadata about a database schema from the given database.
func SchemaMetadata(targetDB db.DB) (schemaMetadata, error) {
	metadata := schemaMetadata{}
	txn, err := targetDB.NewTransaction(false)
	if err != nil {
		return metadata, err
	}
	if err := txn.Get(db.SchemaVersion.Key(), func(b []byte) error {
		metadata.Version = binary.BigEndian.Uint64(b)
		return nil
	}); err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return metadata, utils.RunAndWrapOnError(txn.Discard, err)
	}

	if err := txn.Get(db.SchemaIntermediateState.Key(), func(b []byte) error {
		return cbor.Unmarshal(b, &metadata.IntermediateState)
	}); err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return metadata, utils.RunAndWrapOnError(txn.Discard, err)
	}

	return metadata, txn.Discard()
}

// updateSchemaMetadata updates the schema in given database.
func updateSchemaMetadata(txn db.Transaction, schema schemaMetadata) error {
	var (
		version [8]byte
		state   []byte
		err     error
	)
	binary.BigEndian.PutUint64(version[:], schema.Version)
	state, err = cbor.Marshal(schema.IntermediateState)
	if err != nil {
		return err
	}

	if err := txn.Set(db.SchemaVersion.Key(), version[:]); err != nil {
		return err
	}
	return txn.Set(db.SchemaIntermediateState.Key(), state)
}

func removePendingBlock(txn db.Transaction, _ *utils.Network) error {
	return txn.Delete(db.Unused.Key())
}

func processBlocks(txn db.Transaction, processBlock func(uint64, *sync.Mutex) error) error {
	numOfWorkers := runtime.GOMAXPROCS(0)
	workerPool := pool.New().WithErrors().WithMaxGoroutines(numOfWorkers)

	chainHeight, err := blockchain.ChainHeight(txn)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil
		}
		return err
	}
	blockNumbers := make(chan uint64, 1024) //nolint:mnd
	go func() {
		for bNumber := range chainHeight + 1 {
			blockNumbers <- bNumber
		}
		close(blockNumbers)
	}()
	var txnLock sync.Mutex
	for range numOfWorkers {
		workerPool.Go(func() error {
			for bNumber := range blockNumbers {
				if err := processBlock(bNumber, &txnLock); err != nil {
					return err
				}
			}
			return nil
		})
	}
	return workerPool.Wait()
}

func calculateL1MsgHashes(txn db.Transaction, n *utils.Network) error {
	processBlockFunc := func(blockNumber uint64, txnLock *sync.Mutex) error {
		txnLock.Lock()
		txns, err := blockchain.TransactionsByBlockNumber(txn, blockNumber)
		txnLock.Unlock()
		if err != nil {
			return err
		}
		txnLock.Lock()
		defer txnLock.Unlock()
		return blockchain.StoreL1HandlerMsgHashes(txn, txns)
	}
	return processBlocks(txn, processBlockFunc)
}
