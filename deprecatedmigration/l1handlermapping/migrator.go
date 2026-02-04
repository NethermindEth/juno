package l1handlermapping

import (
	"context"
	"encoding/binary"
	"errors"
	"runtime"
	"time"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	deprecatedprogresslogger "github.com/NethermindEth/juno/deprecatedmigration/progresslogger"
	"github.com/NethermindEth/juno/migration/pipeline"
	progresslogger "github.com/NethermindEth/juno/migration/progresslogger"
	"github.com/NethermindEth/juno/migration/semaphore"
	"github.com/NethermindEth/juno/utils"
)

const (
	// batchByteSize is the initially allocated size of a batch.
	batchByteSize = 128 * utils.Megabyte

	// targetBatchByteSize is the threshold at which a batch is written to the database.
	targetBatchByteSize = 96 * utils.Megabyte

	// logRate is the rate at which we log the progress in block numbers.
	timeLogRate = 30 * time.Second
)

// Migrator recalculates L1 message hash to L2 transaction hash mapping from transactions
type Migrator struct {
	startFrom uint64
}

func (m *Migrator) Before(intermediateState []byte) error {
	if len(intermediateState) >= 8 {
		m.startFrom = binary.BigEndian.Uint64(intermediateState[:8])
	}
	return nil
}

func (m *Migrator) Migrate(
	ctx context.Context,
	database db.KeyValueStore,
	_ *utils.Network,
	log utils.StructuredLogger,
) ([]byte, error) {
	chainHeight, err := core.GetChainHeight(database)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, nil
		}
		return nil, err
	}

	if m.startFrom > 0 {
		log.Info("Resuming L1 handler message hash migration",
			utils.SugaredFields(
				"chain_height", chainHeight,
				"from_block", m.startFrom,
			)...,
		)
	} else {
		log.Info("Starting L1 handler message hash migration",
			utils.SugaredFields("chain_height", chainHeight)...,
		)
	}

	numWorkers := runtime.GOMAXPROCS(0)
	progressTracker := deprecatedprogresslogger.NewBlockProgressTracker(log, chainHeight, m.startFrom)
	resumeFrom, err := migrateBlockRange(
		ctx,
		database,
		log,
		m.startFrom,
		chainHeight,
		numWorkers,
		progressTracker,
	)
	if err != nil {
		return nil, err
	}

	if shouldResume := resumeFrom <= chainHeight; shouldResume {
		log.Info("L1 handler message hash migration interrupted",
			utils.SugaredFields(
				"resume_from", resumeFrom,
				"elapsed", progressTracker.Elapsed(),
			)...,
		)
		return encodeIntermediateState(resumeFrom), nil
	}

	log.Info("L1 handler message hash migration completed",
		utils.SugaredFields("elapsed", progressTracker.Elapsed())...,
	)
	return nil, nil
}

func migrateBlockRange(
	ctx context.Context,
	database db.KeyValueStore,
	logger utils.StructuredLogger,
	startFrom uint64,
	rangeEnd uint64,
	maxWorkers int,
	progressTracker *deprecatedprogresslogger.BlockProgressTracker,
) (uint64, error) {
	loggerCancel := progresslogger.CallEveryInterval(ctx, timeLogRate, progressTracker.LogProgress)
	defer loggerCancel()

	batchSemaphore := semaphore.New(
		maxWorkers+1,
		func() db.Batch {
			return database.NewBatchWithSize(batchByteSize)
		},
	)

	nextBlockNumber := startFrom
	blockNumberSource := pipeline.Source(func(yield func(uint64) bool) {
		for ; nextBlockNumber <= rangeEnd; nextBlockNumber++ {
			if !yield(nextBlockNumber) {
				return
			}
		}
	})

	ingestorPipeline := pipeline.New(
		blockNumberSource,
		maxWorkers,
		newIngestor(
			database,
			batchSemaphore,
			maxWorkers,
			progressTracker,
		),
	)

	committerPipeline := pipeline.New(
		ingestorPipeline,
		1,
		newCommitter(logger, batchSemaphore),
	)

	_, wait := committerPipeline.Run(ctx)
	if res := wait(); res.Err != nil {
		return 0, res.Err
	}

	return nextBlockNumber, nil
}

func encodeIntermediateState(nextBlock uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, nextBlock)
	return buf
}
