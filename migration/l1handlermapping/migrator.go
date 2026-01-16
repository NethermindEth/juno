package l1handlermapping

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
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
	log utils.SimpleLogger, //nolint:staticcheck,nolintlint,lll // ignore staticcheck we are complying with the Migration interface, nolintlint because main config does not checks
) ([]byte, error) {
	chainHeight, err := core.GetChainHeight(database)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, nil
		}
		return nil, err
	}

	startFrom := m.startFrom
	startTime := time.Now()

	if startFrom > 0 {
		log.Infow("Resuming L1 handler message hash migration",
			"chain_height", chainHeight,
			"from_block", startFrom,
		)
	} else {
		log.Infow("Starting L1 handler message hash migration",
			"chain_height",
			chainHeight,
		)
	}

	numWorkers := runtime.GOMAXPROCS(0)
	resumeFrom, err := migrateBlockRange(
		ctx,
		database,
		log,
		startFrom,
		chainHeight,
		numWorkers,
	)
	if err != nil {
		return nil, err
	}

	elapsed := time.Since(startTime)

	if shouldResume := resumeFrom <= chainHeight; shouldResume {
		log.Infow("L1 handler message hash migration interrupted",
			"resume_from", resumeFrom,
			"elapsed", fmt.Sprintf("%.2fs", elapsed.Seconds()),
		)
		return encodeIntermediateState(resumeFrom), nil
	}

	log.Infow("L1 handler message hash migration completed",
		"elapsed", fmt.Sprintf("%.2fs", elapsed.Seconds()),
	)
	return nil, nil
}

func migrateBlockRange(
	ctx context.Context,
	database db.KeyValueStore,
	logger utils.SimpleLogger, //nolint:staticcheck,nolintlint,lll // ignore staticcheck we are complying with the Migration interface, nolintlint because main config does not checks
	startFrom uint64,
	chainHeight uint64,
	maxWorkers int,
) (uint64, error) {
	progressTracker := progresslogger.NewBlockNumberProgressTracker(logger, chainHeight, startFrom)
	loggerCancel := progresslogger.CallEveryInterval(ctx, timeLogRate, progressTracker.LogProgress)
	defer loggerCancel()

	batchSemaphore := semaphore.New(
		maxWorkers+1,
		func() db.Batch {
			return database.NewBatchWithSize(batchByteSize)
		},
	)

	blockNumberCh := make(chan uint64)

	ingestorPipeline := pipeline.New(
		blockNumberCh,
		maxWorkers,
		newIngestor(
			database,
			batchSemaphore,
			maxWorkers,
			progressTracker,
		),
	)

	committerPipeline := pipeline.New(
		ingestorPipeline.Outputs(),
		1,
		newCommitter(logger, batchSemaphore),
	)

	nextBlockNumber := startFrom
outerLoop:
	for ; nextBlockNumber <= chainHeight; nextBlockNumber++ {
		select {
		case <-ctx.Done():
			break outerLoop
		case blockNumberCh <- nextBlockNumber:
		}
	}
	close(blockNumberCh)

	// Wait for all ingestors to finish, because we closed their inputs [blockNumberCh].
	// Then their output channel is closed
	if err := ingestorPipeline.Wait(); err != nil {
		return 0, err
	}

	// Wait for the committer to finish, because we closed its input [ingestorPipeline.outputs].
	// Then its output channel is closed
	if err := committerPipeline.Wait(); err != nil {
		return 0, err
	}

	return nextBlockNumber, nil
}

func encodeIntermediateState(nextBlock uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, nextBlock)
	return buf
}
