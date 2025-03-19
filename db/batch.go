package db

// A write-only store that gathers changes in-memory and writes them to disk in a single atomic operation.
// It is not thread-safe for a single batch, but different batches can be used in different threads.
type Batch interface {
	KeyValueWriter
	// Retrieves the value size of the data stored in the batch for writing
	Size() int
	// Flushes the data stored to disk
	Write() error
	// Resets the batch
	Reset()
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
// Write operations will be slower.
// Ideally, IndexedBatch should not be used at all, because it's better to write using regular batch
// and read directly from the database.
type IndexedBatch interface {
	Batch
	KeyValueReader
	Iterable
}

// Produce an IndexedBatch to write to the database and read from it.
type IndexedBatcher interface {
	NewIndexedBatch() IndexedBatch
	NewIndexedBatchWithSize(size int) IndexedBatch
}
