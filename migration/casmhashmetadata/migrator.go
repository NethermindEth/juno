package casmhashmetadata

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"runtime"
	"sort"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/typed"
	"github.com/NethermindEth/juno/db/typed/key"
	"github.com/NethermindEth/juno/db/typed/value"
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

	// timeLogRate is the rate at which we log the progress.
	timeLogRate = 10 * time.Second
)

type Migrator struct {
	startFrom uint64
}

// This migration migrates from the old CASM hash V2 bucket format to the new metadata format.
// deprecatedCasmHashV2Bucket is the old accessor used to read pre-migration data.
var deprecatedCasmHashV2Bucket = typed.NewBucket(
	db.ClassCasmHashMetadata,
	key.SierraClassHash,
	value.CasmClassHash,
)

func (m *Migrator) Before(intermediateState []byte) error {
	if len(intermediateState) >= 8 {
		m.startFrom = binary.BigEndian.Uint64(intermediateState[:8])
	}
	return nil
}

func (m *Migrator) Migrate(
	ctx context.Context,
	database db.KeyValueStore,
	network *utils.Network,
	log utils.SimpleLogger, //nolint:staticcheck,nolintlint,lll // ignore staticcheck we are complying with the Migration interface, nolintlint because main config does not checks
) ([]byte, error) {
	startTime := time.Now()
	chainHeight, err := core.GetChainHeight(database)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, nil // empty db nothing to process
		}
		return nil, err
	}

	if m.startFrom > 0 {
		log.Infow("Resuming Casm hash metadata migration",
			"chain_height", chainHeight,
			"resuming_from", m.startFrom,
		)
	} else {
		log.Infow("Starting Casm hash metadata migration",
			"chain_height",
			chainHeight,
		)
	}

	cutoff := firstBlockWithProtocolVersionGreaterThanOrEqual(
		database,
		core.Ver0_14_1,
		chainHeight,
	)
	maxWorkers := runtime.GOMAXPROCS(0)
	// setup progress tracker and logger
	progressTracker := progresslogger.NewBlockNumberProgressTracker(log, chainHeight, m.startFrom)
	loggerCancel := progresslogger.CallEveryInterval(ctx, timeLogRate, progressTracker.LogProgress)
	defer loggerCancel()

	batchSemaphore := semaphore.New(maxWorkers, func() db.Batch {
		return database.NewBatchWithSize(int(batchByteSize))
	})
	// Phase 1: Process blocks before 0.14.1
	if m.startFrom < cutoff {
		toBlock := cutoff - 1
		processor := newPre0141Ingestor(
			database,
			batchSemaphore,
			maxWorkers,
			progressTracker,
		)

		resumeFrom, err := migrateRange(
			ctx,
			log,
			batchSemaphore,
			m.startFrom,
			toBlock,
			maxWorkers,
			processor,
		)
		if err != nil {
			return nil, err
		}

		elapsed := time.Since(startTime)
		// Check for cancellation after processing
		if shouldResume := resumeFrom <= toBlock; shouldResume {
			log.Infow("Casm hash metadata migration interrupted",
				"resume_from", resumeFrom,
				"elapsed", fmt.Sprintf("%.2fs", elapsed.Seconds()),
			)
			return encodeIntermediateState(resumeFrom), nil
		}
	}

	// Phase 2: Process blocks from 0.14.1 onwards
	phase2Start := max(cutoff, m.startFrom)
	if phase2Start <= chainHeight {
		processor := newPost0141Ingestor(
			database,
			batchSemaphore,
			maxWorkers,
			progressTracker,
		)
		resumeFrom, err := migrateRange(
			ctx,
			log,
			batchSemaphore,
			phase2Start,
			chainHeight,
			maxWorkers,
			processor,
		)
		if err != nil {
			return nil, err
		}

		elapsed := time.Since(startTime)
		if shouldResume := resumeFrom <= chainHeight; shouldResume {
			log.Infow("Casm hash metadata migration interrupted",
				"resume_from", resumeFrom,
				"elapsed", fmt.Sprintf("%.2fs", elapsed.Seconds()),
			)
			return encodeIntermediateState(resumeFrom), nil
		}
	}

	log.Infow("Casm hash metadata migration completed",
		"elapsed", fmt.Sprintf("%.2fs", time.Since(startTime).Seconds()),
	)

	return nil, nil
}

func migrateRange(
	ctx context.Context,
	log utils.SimpleLogger, //nolint:staticcheck,nolintlint,lll // ignore staticcheck we are complying with the Migration interface, nolintlint because main config does not checks
	batchSemaphore semaphore.ResourceSemaphore[db.Batch],
	fromBlock, toBlock uint64,
	maxWorkers int,
	processor pipeline.State[uint64, db.Batch],
) (uint64, error) {
	blockNumbers := make(chan uint64)

	ingestorPipeline := pipeline.New(blockNumbers, maxWorkers, processor)
	committerPipeline := pipeline.New(
		ingestorPipeline.Outputs(),
		maxWorkers,
		newCommitter(log, batchSemaphore),
	)

	nextBlockNumber := fromBlock
outerLoop:
	for ; nextBlockNumber <= toBlock; nextBlockNumber++ {
		select {
		case <-ctx.Done():
			break outerLoop
		case blockNumbers <- nextBlockNumber:
		}
	}
	close(blockNumbers)

	if err := ingestorPipeline.Wait(); err != nil {
		return 0, err
	}
	if err := committerPipeline.Wait(); err != nil {
		return 0, err
	}

	return nextBlockNumber, nil
}

func firstBlockWithProtocolVersionGreaterThanOrEqual(
	database db.KeyValueStore, version *semver.Version, rangeEnd uint64,
) uint64 {
	// Binary search to find the first block with protocol version
	cutoffBlock := sort.Search(int(rangeEnd+1), func(i int) bool {
		header, err := core.GetBlockHeaderByNumber(database, uint64(i))
		if err != nil {
			return false // If we can't get the header, assume it's not the cutoff
		}

		blockVer, err := core.ParseBlockVersion(header.ProtocolVersion)
		if err != nil {
			return false // If we can't parse, assume it's not the cutoff
		}

		return blockVer.GreaterThanEqual(version)
	})
	return uint64(cutoffBlock)
}

func encodeIntermediateState(nextBlock uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, nextBlock)
	return buf
}
