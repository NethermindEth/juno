package casmhashmetadata

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"runtime"
	"sort"
	"sync/atomic"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/typed"
	"github.com/NethermindEth/juno/db/typed/key"
	"github.com/NethermindEth/juno/db/typed/value"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/utils"
)

const timeLogRate = 15 * time.Second

type CasmHashMetadataMigration struct {
	startFrom uint64
}

// This migration migrates from the old CASM hash V2 bucket format to the new metadata format.
// deprecatedCasmHashV2Bucket is the old accessor used to read pre-migration data.
var deprecatedCasmHashV2Bucket = typed.NewBucket(
	db.ClassCasmHashMetadata,
	key.SierraClassHash,
	value.CasmClassHash,
)

func (m *CasmHashMetadataMigration) Before(intermediateState []byte) error {
	if len(intermediateState) >= 8 {
		m.startFrom = binary.BigEndian.Uint64(intermediateState[:8])
	}
	return nil
}

func (m *CasmHashMetadataMigration) Migrate(
	ctx context.Context,
	database db.KeyValueStore,
	network *utils.Network,
	log utils.SimpleLogger, //nolint:staticcheck,nolintlint
) ([]byte, error) {
	chainHeight, err := core.GetChainHeight(database)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, nil // empty db nothing to process
		}
		return nil, err
	}

	if m.startFrom > 0 {
		log.Infow("Class CASM hash metadata migration started",
			"resuming_from", m.startFrom,
			"total_blocks", chainHeight+1,
		)
	} else {
		log.Infow("Class CASM hash metadata migration started",
			"total_blocks", chainHeight+1,
		)
	}

	var blocksProcessed atomic.Uint64
	blocksProcessed.Store(m.startFrom)
	cutoff := firstBlockWithProtocolVersionGreaterThanOrEqual(
		database,
		core.Ver0_14_1,
		chainHeight,
	)
	maxWorkers := runtime.GOMAXPROCS(0)

	stopProgressLogger := startProgressLogger(ctx, log, &blocksProcessed, chainHeight+1)
	defer stopProgressLogger()

	// Phase 1: Process blocks before 0.14.1
	if m.startFrom < cutoff {
		batch := database.NewBatch()
		processor := &pre0141Processor{
			database:        database,
			batch:           batch,
			blocksProcessed: &blocksProcessed,
		}

		err := processRange(ctx, m.startFrom, cutoff-1, maxWorkers, processor)
		if err != nil {
			return nil, err
		}

		// write first batch so second phase can read from it
		if err := batch.Write(); err != nil {
			return nil, fmt.Errorf("failed to commit phase 1 batch: %w", err)
		}

		// Check for cancellation after processing
		if ctx.Err() != nil {
			log.Infow("Class CASM hash metadata migration interrupted",
				"blocks_processed", blocksProcessed.Load(),
			)
			return encodeIntermediateState(blocksProcessed.Load()), ctx.Err()
		}
	}

	// Phase 2: Process blocks from 0.14.1 onwards
	phase2Start := max(cutoff, m.startFrom)
	if phase2Start <= chainHeight {
		batch := database.NewBatch()
		processor := &post0141Processor{
			database:        database,
			batch:           batch,
			blocksProcessed: &blocksProcessed,
		}

		err := processRange(ctx, phase2Start, chainHeight, maxWorkers, processor)
		if err != nil {
			return nil, err
		}

		if err := batch.Write(); err != nil {
			return nil, fmt.Errorf("failed to commit phase 2 batch: %w", err)
		}

		// Check for cancellation after processing
		if ctx.Err() != nil {
			log.Infow("Class CASM hash metadata migration interrupted",
				"blocks_processed", blocksProcessed.Load(),
			)
			return encodeIntermediateState(blocksProcessed.Load()), ctx.Err()
		}
	}

	log.Infow("Class CASM hash metadata migration completed",
		"blocks_processed", blocksProcessed.Load(),
	)

	return nil, nil
}

func processRange(
	ctx context.Context,
	fromBlock, toBlock uint64,
	maxWorkers int,
	processor pipeline.State[uint64, struct{}],
) error {
	blockNumbers := make(chan uint64)
	go func() {
		defer close(blockNumbers)
		for bNumber := fromBlock; bNumber <= toBlock; bNumber++ {
			select {
			case <-ctx.Done():
				return
			case blockNumbers <- bNumber:
			}
		}
	}()

	p := pipeline.New(blockNumbers, maxWorkers, processor)
	return p.Wait()
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
	intermediateState := make([]byte, 8)
	binary.BigEndian.PutUint64(intermediateState[:8], nextBlock)
	return intermediateState
}

// startProgressLogger starts a goroutine that logs migration progress every 30 seconds.
// It returns a function to stop the logger.
func startProgressLogger(
	ctx context.Context,
	log utils.SimpleLogger, //nolint:staticcheck,nolintlint
	blocksProcessed *atomic.Uint64,
	totalBlocks uint64,
) func() {
	stopCh := make(chan struct{})
	ticker := time.NewTicker(timeLogRate)

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stopCh:
				return
			case <-ticker.C:
				current := blocksProcessed.Load()
				var percentage float64
				if totalBlocks > 0 {
					percentage = float64(current) / float64(totalBlocks) * 100
				}
				remaining := totalBlocks - current

				log.Infow("Class CASM hash metadata migration progress",
					"blocks_processed", current,
					"blocks_remaining", remaining,
					"percentage", fmt.Sprintf("%.2f%%", percentage),
				)
			}
		}
	}()

	return func() {
		close(stopCh)
	}
}
