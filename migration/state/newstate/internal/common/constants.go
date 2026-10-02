package common

import (
	"time"

	"github.com/NethermindEth/juno/db"
)

const (
	BatchByteSize = 128 * db.Megabyte
	// TODO: this is per-ingestor and common for all phases,
	// but different phases may have different memory load.
	// On Sepolia headstate phase accumulates 4 x 64 MB and never reaches the threshold:
	// one commit at the very end, no crash safety, no progress output
	TargetBatchByteSize = 96 * db.Megabyte
	IngestorCount       = 4
	TimeLogRate         = 5 * time.Second
)
