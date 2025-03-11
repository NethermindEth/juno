package db

type Batch interface {
	KeyValueWriter
	// Retrieves the value size of the data stored in the batch for writing
	Size() int
	// Flushes the data stored to disk
	Write() error
	// Resets the batch
	Reset()
}

type Batcher interface {
	// Creates a write-only batch
	NewBatch() Batch
	// Creates a write-only batch with a pre-allocated size
	NewBatchWithSize(size int) Batch
}
