package db

import (
	"io"

	"github.com/NethermindEth/juno/utils"
)

const DefaultBatchSize = 10 * utils.Megabyte

// A write-only store that gathers changes in-memory and writes them to disk in a single atomic operation.
// It is not thread-safe for a single batch, but different batches can be used in different threads.
type Batch interface {
	io.Closer
	KeyValueWriter
	KeyValueRangeDeleter
	// Retrieves the value size of the data stored in the batch for writing
	Size() int
	// Flushes the data stored to disk and closes the batch
	Write() error
}

// Produce a batch to write to the database
type Batcher interface {
	// Creates a write-only batch
	NewBatch() Batch
	// Creates a write-only batch with a pre-allocated size
	NewBatchWithSize(size int) Batch
}

// Same as Batch, but allows for reads from the batch and the disk.
// Use this only if you need to read from both the in-memory and on-disk data.
// Write operations will be slower compared to a regular Batch.
//
// Deprecated: Use Batch for writes and access the database directly for reads.
type IndexedBatch interface {
	Batch
	KeyValueReader
}

// Produce an IndexedBatch to write to the database and read from it.
//
// Deprecated: Use Batcher for writes and access the database directly for reads.
type IndexedBatcher interface {
	// Deprecated: Use NewBatch instead.
	NewIndexedBatch() IndexedBatch
	// Deprecated: Use NewBatchWithSize instead.
	NewIndexedBatchWithSize(size int) IndexedBatch
}
