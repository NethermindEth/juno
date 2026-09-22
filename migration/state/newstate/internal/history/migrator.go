package history

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"time"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/dbutils"
	"github.com/NethermindEth/juno/migration"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/semaphore"
	"github.com/NethermindEth/juno/migration/state/newstate/internal/common"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

var _ migration.Migration = (*Migrator)(nil)

// Intermediate state is [phase:1][addr:32]: the phase that was interrupted
// and the address it resumes at. The phases run strictly in order, so the
// ones before it are complete and the ones after it have not started.
const intermediateStateLen = 1 + felt.Bytes

// Migrator rewrites the contract history layout so each entry stores the
// post-update value at its block, instead of the pre-update value.
//
// Example — a contract whose class hash was 0xAA at deploy (block 100),
// changed to 0xBB at block 200, then to 0xCC at block 500:
//
//	block │ old layout (pre-value) │ new layout (post-value)
//	──────┼────────────────────────┼─────────────────────────
//	100   │ (no entry)             │ 0xAA  ← explicit deploy
//	200   │ 0xAA                   │ 0xBB
//	500   │ 0xBB                   │ 0xCC
//	head  │ 0xCC (contract record) │ (read from history)
//
// The same shape change applies to nonces and per-slot storage. The migrator
// runs three phases (class-hash, nonce, storage); each walks the Contract
// bucket in address order and rewrites one contract's deprecated entries at a
// time, then drops the whole deprecated bucket in one range delete.
//
// Cancellation stops a phase's source but not its workers or committer, so
// every contract already handed out is finished and written. The source's
// position is therefore an exact resume point: Migrate returns it and Before
// picks it up. Each new entry is a pure function of the deprecated rows, which
// survive until the phase's wipe, so a contract rewritten twice comes out
// identical.
type Migrator struct {
	// Set by Before. Zero values mean a fresh run: the first phase from the
	// zero address, which seeks to the first contract.
	phase      uint8
	resumeFrom felt.Address
}

func (m *Migrator) Before(state []byte) error {
	if len(state) == 0 {
		*m = Migrator{}
		return nil
	}
	if len(state) != intermediateStateLen {
		return fmt.Errorf(
			"history: intermediate state is %d bytes, want %d", len(state), intermediateStateLen,
		)
	}
	if int(state[0]) >= len(phases) {
		return fmt.Errorf(
			"history: intermediate state names phase %d, but only %d exist", state[0], len(phases),
		)
	}
	m.phase = state[0]
	m.resumeFrom = felt.FromBytes[felt.Address](state[1:])
	return nil
}

func encodeState(phase uint8, resumeFrom *felt.Address) []byte {
	state := make([]byte, intermediateStateLen)
	state[0] = phase
	addrBytes := resumeFrom.Bytes()
	copy(state[1:], addrBytes[:])
	return state
}

type newIngestorFunc = func(
	semaphore.ResourceSemaphore[db.Batch], db.KeyValueReader,
) pipeline.State[felt.Address, common.Task]

// phase is one rewrite over the whole Contract bucket.
type phase struct {
	name        string
	deprecated  db.Bucket
	newIngestor newIngestorFunc
}

var phases = [...]phase{
	{
		name:       "class-hash",
		deprecated: db.DeprecatedContractClassHashHistory,
		newIngestor: func(
			sem semaphore.ResourceSemaphore[db.Batch], r db.KeyValueReader,
		) pipeline.State[felt.Address, common.Task] {
			return newClassHashIngestor(sem, r)
		},
	},
	{
		name:       "nonce",
		deprecated: db.DeprecatedContractNonceHistory,
		newIngestor: func(
			sem semaphore.ResourceSemaphore[db.Batch], r db.KeyValueReader,
		) pipeline.State[felt.Address, common.Task] {
			return newNonceIngestor(sem, r)
		},
	},
	{
		name:       "storage",
		deprecated: db.DeprecatedContractStorageHistory,
		newIngestor: func(
			sem semaphore.ResourceSemaphore[db.Batch], r db.KeyValueReader,
		) pipeline.State[felt.Address, common.Task] {
			return newStorageIngestor(sem, r)
		},
	},
}

// Migrate returns (state, nil) when interrupted, (nil, nil) when complete and
// (nil, err) on failure, which reruns from the last persisted state.
func (m *Migrator) Migrate(
	ctx context.Context,
	database db.KeyValueStore,
	_ *networks.Network,
	logger log.StructuredLogger,
) ([]byte, error) {
	if m.phase > 0 || m.resumeFrom != (felt.Address{}) {
		logger.Info("Resuming history migration",
			zap.Strings("completedPhases", completedPhaseNames(m.phase)),
			zap.String("phase", phases[m.phase].name),
			zap.String("resumeFrom", m.resumeFrom.String()),
		)
	}

	for i := int(m.phase); i < len(phases); i++ {
		var progress felt.Address
		if i == int(m.phase) {
			progress = m.resumeFrom
		}

		done, err := runPhase(ctx, database, logger, &phases[i], &progress)
		if err != nil {
			return nil, err
		}
		if !done {
			logger.Info("History migration interrupted",
				zap.String("phase", phases[i].name),
				zap.String("resumeFrom", progress.String()),
			)
			return encodeState(uint8(i), &progress), nil
		}
	}
	return nil, nil
}

// completedPhaseNames names the phases wiped by an earlier run.
func completedPhaseNames(phase uint8) []string {
	names := make([]string, phase)
	for i := range names {
		names[i] = phases[i].name
	}
	return names
}

// runPhase rewrites one bucket, walking contracts from *progress. If the walk
// is interrupted the source sets *progress to the first address never handed
// out — where a rerun resumes. done reports whether the walk reached the end;
// only then is the deprecated bucket wiped.
func runPhase(
	ctx context.Context,
	database db.KeyValueStore,
	logger log.StructuredLogger,
	p *phase,
	progress *felt.Address,
) (bool, error) {
	sem := semaphore.New(common.IngestorCount+1, func() db.Batch {
		return database.NewBatchWithSize(common.BatchByteSize)
	})
	// The drain that follows cancellation is silent and can take a while; say so.
	stopLog := context.AfterFunc(ctx, func() {
		logger.Info("Cancelled, finishing the contracts already handed out",
			zap.String("phase", p.name),
		)
	})
	defer stopLog()

	seq, sourceErr := addressSeq(database, progress)
	ingestors := pipeline.New(pipeline.Source(seq), common.IngestorCount, p.newIngestor(sem, database))
	committers := pipeline.New(ingestors, 1, common.NewCommitter(logger, sem, p.name))

	start := time.Now()
	_, wait := committers.Run(ctx)
	res := wait()

	if err := errors.Join(sourceErr(), res.Err); err != nil {
		return false, fmt.Errorf("%s: %w", p.name, err)
	}
	if !res.IsDone {
		if ctx.Err() == nil {
			return false, fmt.Errorf("%s phase stopped without being cancelled", p.name)
		}
		return false, nil
	}

	logger.Info("Applied history phase",
		zap.String("phase", p.name),
		zap.Duration("elapsed", time.Since(start)),
	)
	return true, wipeDeprecated(database, p.deprecated)
}

// wipeDeprecated drops a whole deprecated bucket in one range tombstone. Deleting
// per contract instead left the compactor millions of tombstones to resolve.
func wipeDeprecated(database db.KeyValueStore, bucket db.Bucket) error {
	start := bucket.Key()
	if err := database.DeleteRange(start, dbutils.UpperBound(start)); err != nil {
		return fmt.Errorf("wiping %s: %w", bucket, err)
	}
	return nil
}

// addressSeq yields the Contract bucket's addresses in order, starting at
// *progress. When yield refuses an address — the pipeline was cancelled — it
// writes that address into *progress: the first one never handed out. A walk
// that runs out leaves *progress untouched.
func addressSeq(
	r db.KeyValueReader, progress *felt.Address,
) (iter.Seq[felt.Address], func() error) {
	var iterErr error
	seq := func(yield func(felt.Address) bool) {
		prefix := db.Contract.Key()
		it, err := r.NewIterator(prefix, true)
		if err != nil {
			iterErr = err
			return
		}
		defer it.Close()

		progressBytes := progress.Bytes()
		from := append(append(make([]byte, 0, len(prefix)+felt.Bytes), prefix...), progressBytes[:]...)
		for valid := it.Seek(from); valid; valid = it.Next() {
			key := it.Key()
			if len(key) != len(prefix)+felt.Bytes {
				iterErr = fmt.Errorf(
					"malformed Contract key: len %d, want %d",
					len(key),
					len(prefix)+felt.Bytes,
				)
				return
			}
			addr := felt.FromBytes[felt.Address](key[len(prefix):])
			if !yield(addr) {
				*progress = addr
				return
			}
		}
	}
	return seq, func() error { return iterErr }
}
