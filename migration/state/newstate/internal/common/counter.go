package common

import (
	"math"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

type Counter struct {
	logger         log.StructuredLogger
	timeLogRate    time.Duration
	phaseName      string
	start          time.Time
	size           uint64
	completedAddrs uint64
	entryCount     uint64
}

func NewCounter(logger log.StructuredLogger, timeLogRate time.Duration, phaseName string) Counter {
	if zl, ok := logger.(*log.ZapLogger); ok {
		logger = zl.WithOptions(zap.AddCallerSkip(1))
	}
	return Counter{
		logger:      logger,
		timeLogRate: timeLogRate,
		phaseName:   phaseName,
		start:       time.Now(),
	}
}

func (c *Counter) Log(byteSize uint64, completedAddrs, entryCount int) {
	c.size += byteSize
	c.completedAddrs += uint64(completedAddrs)
	c.entryCount += uint64(entryCount)

	const cent = 100

	now := time.Now()
	elapsed := now.Sub(c.start).Seconds()
	if elapsed <= c.timeLogRate.Seconds() {
		return
	}

	mbs := float64(c.size) / float64(db.Megabyte)
	fields := make([]zap.Field, 0, 8)
	if c.phaseName != "" {
		fields = append(fields, zap.String("phase", c.phaseName))
	}
	fields = append(fields,
		zap.Float64("MB", math.Round(mbs*cent)/cent),
		zap.Float64("MB/s", math.Round(mbs/elapsed*cent)/cent),
		zap.Uint64("completedContracts", c.completedAddrs),
		zap.Float64("completedContracts/s", float64(c.completedAddrs)/elapsed),
		zap.Uint64("entries", c.entryCount),
		zap.Float64("entries/s", float64(c.entryCount)/elapsed),
		zap.Float64("time", elapsed),
	)
	c.logger.Info("write speed", fields...)

	c.start = now
	c.size = 0
	c.completedAddrs = 0
	c.entryCount = 0
}
