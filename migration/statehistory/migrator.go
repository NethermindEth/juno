package statehistory

import (
	"context"
	"errors"
	"fmt"
	"iter"
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
	batch          db.Batch
	completedAddrs int
	entryCount     int
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
	if err := runClassHashPhase(ctx, database, logger); err != nil {
		return shouldRerun, err
	}
	if err := runNoncePhase(ctx, database, logger); err != nil {
		return shouldRerun, err
	}
	if err := runStoragePhase(ctx, database, logger); err != nil {
		return shouldRerun, err
	}

	return shouldNotRerun, nil
}

func runClassHashPhase(
	ctx context.Context,
	database db.KeyValueStore,
	logger log.StructuredLogger,
) error {
	sem, src, sourceErr := setupBeforePhase(database)
	ing := newClassHashIngestor(ctx, sem, database)
	return runPipeline(ctx, "class-hash", src, ing, logger, sem, sourceErr)
}

func runNoncePhase(
	ctx context.Context,
	database db.KeyValueStore,
	logger log.StructuredLogger,
) error {
	sem, src, sourceErr := setupBeforePhase(database)
	ing := newNonceIngestor(ctx, sem, database)
	return runPipeline(ctx, "nonce", src, ing, logger, sem, sourceErr)
}

func runStoragePhase(
	ctx context.Context,
	database db.KeyValueStore,
	logger log.StructuredLogger,
) error {
	sem, src, sourceErr := setupBeforePhase(database)
	ing := newStorageIngestor(ctx, sem, database)
	return runPipeline(ctx, "storage", src, ing, logger, sem, sourceErr)
}

func setupBeforePhase(
	database db.KeyValueStore,
) (semaphore.ResourceSemaphore[db.Batch], pipeline.Pipeline[*felt.Felt], func() error) {
	sem := semaphore.New(ingestorCount+1, func() db.Batch {
		return database.NewBatchWithSize(batchByteSize)
	})
	seq, sourceErr := addressSeq(database)
	return sem, pipeline.Source(seq), sourceErr
}

func runPipeline(
	ctx context.Context,
	name string,
	src pipeline.Pipeline[*felt.Felt],
	ing pipeline.State[*felt.Felt, task],
	logger log.StructuredLogger,
	sem semaphore.ResourceSemaphore[db.Batch],
	sourceErr func() error,
) error {
	ingestors := pipeline.New(src, ingestorCount, ing)
	committers := pipeline.New(ingestors, 1, newCommitter(logger, sem, name))

	_, wait := committers.Run(ctx)
	res := wait()

	if err := errors.Join(sourceErr(), res.Err); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	if !res.IsDone {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("%s: %w", name, ctxErr)
		}
		return fmt.Errorf("%s phase did not complete", name)
	}
	return nil
}

func addressSeq(r db.KeyValueReader) (iter.Seq[*felt.Felt], func() error) {
	var iterErr error
	seq := func(yield func(*felt.Felt) bool) {
		prefix := db.Contract.Key()
		it, err := r.NewIterator(prefix, true)
		if err != nil {
			iterErr = err
			return
		}
		defer it.Close()
		for valid := it.First(); valid; valid = it.Next() {
			key := it.Key()
			if len(key) != len(prefix)+felt.Bytes {
				iterErr = fmt.Errorf(
					"malformed Contract key: len %d, want %d",
					len(key),
					len(prefix)+felt.Bytes,
				)
				return
			}
			f := felt.FromBytes[felt.Felt](key[len(prefix):])
			if !yield(&f) {
				return
			}
		}
	}
	return seq, func() error { return iterErr }
}
