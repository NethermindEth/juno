package statehistory

import (
	"context"
	"fmt"
	"time"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/semaphore"
	"github.com/NethermindEth/juno/utils/log"
)

const (
	batchByteSize       = 128 * db.Megabyte
	targetBatchByteSize = 96 * db.Megabyte
	ingestorCount       = 4
	timeLogRate         = 5 * time.Second
)

type task struct {
	batch      db.Batch
	addrCount  int
	entryCount int
}

var (
	shouldRerun    = []byte{}
	shouldNotRerun = []byte(nil)
)

var _ migration.Migration = (*Migrator)(nil)

type Migrator struct{}

func (Migrator) Before([]byte) error { return nil }

func (Migrator) Migrate(
	ctx context.Context,
	database db.KeyValueStore,
	_ *networks.Network,
	logger log.StructuredLogger,
) ([]byte, error) {
	addresses, err := collectAddresses(database)
	if err != nil {
		return shouldRerun, err
	}
	if len(addresses) == 0 {
		logger.Info("state history migration: Contract bucket empty, marking applied")
		return shouldNotRerun, nil
	}

	if err := runPhase(
		ctx,
		database,
		logger,
		addresses,
		"class-hash",
		TransformClassHashHistory,
	); err != nil {
		return shouldRerun, err
	}
	if err := runPhase(ctx, database, logger, addresses, "nonce", TransformNonceHistory); err != nil {
		return shouldRerun, err
	}
	if err := runPhase(
		ctx,
		database,
		logger,
		addresses,
		"storage",
		TransformStorageHistory,
	); err != nil {
		return shouldRerun, err
	}

	return shouldNotRerun, nil
}

func runPhase(
	ctx context.Context,
	database db.KeyValueStore,
	logger log.StructuredLogger,
	addresses []felt.Address,
	name string,
	transform func(db.KeyValueReader, *task, felt.Address, FlushBatchFn) error,
) error {
	sem := semaphore.New(ingestorCount+1, func() db.Batch {
		return database.NewBatchWithSize(batchByteSize)
	})
	src := pipeline.Source(func(yield func(felt.Address) bool) {
		for _, a := range addresses {
			if !yield(a) {
				return
			}
		}
	})
	ingestors := pipeline.New(src, ingestorCount, newIngestor(sem, database, transform))
	committers := pipeline.New(ingestors, 1, newCommitter(logger, sem, name))

	_, wait := committers.Run(ctx)
	res := wait()
	if res.Err != nil {
		return res.Err
	}
	if !res.IsDone {
		return fmt.Errorf("%s phase did not complete", name)
	}
	return nil
}

func collectAddresses(r db.KeyValueReader) ([]felt.Address, error) {
	prefix := db.Contract.Key()
	it, err := r.NewIterator(prefix, true)
	if err != nil {
		return nil, err
	}
	defer it.Close()

	var addrs []felt.Address
	for valid := it.First(); valid; valid = it.Next() {
		key := it.Key()
		if len(key) != len(prefix)+felt.Bytes {
			continue
		}
		f := felt.FromBytes[felt.Felt](key[len(prefix):])
		addrs = append(addrs, felt.Address(f))
	}
	return addrs, nil
}
