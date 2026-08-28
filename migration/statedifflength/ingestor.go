package statedifflength

import (
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/progresslogger"
	"github.com/NethermindEth/juno/migration/semaphore"
)

// worker is one pipeline reader's state: the batch it is filling, and the
// commitments record it decodes each block into. It outlives the batches it fills,
// so the reused memory survives a flush.
type worker struct {
	batch db.Batch
	// commitments is decoded into once per block instead of being allocated per
	// block. It is safe to hand to core.WriteBlockCommitment because that encodes it
	// immediately and the batch copies the encoded bytes, so no block's value outlives
	// the call that wrote it.
	commitments core.BlockCommitments
}

// ingestor reads blocks and fills per-worker batches. Each pipeline worker owns one
// entry in workers, so no synchronisation is needed between them.
type ingestor struct {
	database  db.KeyValueReader
	semaphore semaphore.ResourceSemaphore[db.Batch]
	progress  *progresslogger.BlockProgressTracker
	workers   []worker
}

var _ pipeline.State[uint64, db.Batch] = (*ingestor)(nil)

func newIngestor(
	database db.KeyValueReader,
	batchSemaphore semaphore.ResourceSemaphore[db.Batch],
	workerCount int,
	progress *progresslogger.BlockProgressTracker,
) *ingestor {
	workers := make([]worker, workerCount)
	for i := range workers {
		workers[i].batch = batchSemaphore.GetBlocking()
	}
	return &ingestor{
		database:  database,
		semaphore: batchSemaphore,
		progress:  progress,
		workers:   workers,
	}
}

func (in *ingestor) Run(index int, blockNumber uint64, outputs chan<- db.Batch) error {
	cur := &in.workers[index]

	if err := cur.backfillBlock(in.database, cur.batch, blockNumber); err != nil {
		return err
	}
	in.progress.IncrementCompletedBlocks(1)

	if cur.batch.Size() >= targetBatchByteSize {
		outputs <- cur.batch
		cur.batch = in.semaphore.GetBlocking()
	}
	return nil
}

func (in *ingestor) Done(index int, outputs chan<- db.Batch) error {
	outputs <- in.workers[index].batch
	return nil
}

// backfillBlock derives the state diff length from the block's state update and
// writes the block's commitments with it into batch. A missing commitments or
// state update record above the pruned prefix means the two buckets disagree, i.e.
// the database is corrupt, so the read error is returned rather than skipped.
func (w *worker) backfillBlock(
	r db.KeyValueReader,
	batch db.KeyValueWriter,
	blockNumber uint64,
) error {
	if err := w.readCommitments(r, blockNumber); err != nil {
		return err
	}

	length, err := readStateDiffLength(r, blockNumber)
	if err != nil {
		return err
	}

	// Assigned on every block, never read back, which is what makes reusing the struct
	// safe: the decoder leaves a field absent from the stored record untouched, and
	// StateDiffLength is exactly the field the records being migrated do not have.
	w.commitments.StateDiffLength = length
	return core.WriteBlockCommitment(batch, blockNumber, &w.commitments)
}

// readCommitments decodes the block's commitments into the worker's own struct.
// core.GetBlockCommitmentByBlockNum allocates a struct and a felt per commitment on
// every call, whereas decoding into memory that already holds felts reuses them and
// allocates nothing. TestReadCommitmentsMatchesAccessor holds the two paths together.
func (w *worker) readCommitments(r db.KeyValueReader, blockNumber uint64) error {
	err := r.Get(db.BlockCommitmentsKey(blockNumber), func(data []byte) error {
		return encoder.Unmarshal(data, &w.commitments)
	})
	if err != nil {
		return fmt.Errorf("getting commitments for block %d: %w", blockNumber, err)
	}
	return nil
}

// readStateDiffLength counts the block's state diff entries straight out of the
// stored encoding. Decoding the state update instead would allocate a map entry and
// a felt per diff entry only to throw them away, which dominates the migration.
func readStateDiffLength(r db.KeyValueReader, blockNumber uint64) (uint64, error) {
	var length uint64
	err := r.Get(db.StateUpdateByBlockNumKey(blockNumber), func(data []byte) error {
		var err error
		length, err = stateDiffLength(data)
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("counting state diff for block %d: %w", blockNumber, err)
	}
	return length, nil
}
