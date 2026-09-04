package common

import (
	"time"

	"github.com/NethermindEth/juno/db"
)

const (
	BatchByteSize       = 128 * db.Megabyte
	TargetBatchByteSize = 96 * db.Megabyte
	IngestorCount       = 4
	TimeLogRate         = 5 * time.Second
)
