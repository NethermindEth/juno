package common

import (
	"time"

	"github.com/NethermindEth/juno/db"
)

const (
	// Sized against pebble's large-batch threshold, not against Batch.Size().
	// The memtable is a 256 MB arena (--db-memtable-size) of skiplist nodes: each
	// entry costs a 184-byte node plus key, value and an 8-byte trailer, while
	// Batch.Size() counts only key and value. A batch whose arena footprint
	// reaches half the memtable is written as its own flushable and rotates the
	// memtable; in counted bytes that is ~45 MB for storage and ~35 MB for nonce
	// and class-hash. The previous 96 MB target tripped it on every commit;
	// 32 MB stays under it, so two or three batches share one memtable.
	BatchByteSize       = 48 * db.Megabyte
	TargetBatchByteSize = 32 * db.Megabyte
	IngestorCount       = 4
	TimeLogRate         = 5 * time.Second
)
