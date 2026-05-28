package headstate

import (
	"math"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

type counter struct {
	logger         log.StructuredLogger
	timeLogRate    time.Duration
	start          time.Time
	size           uint64
	completedAddrs uint64
}

func newCounter(logger log.StructuredLogger, timeLogRate time.Duration) counter {
	if zl, ok := logger.(*log.ZapLogger); ok {
		logger = zl.WithOptions(zap.AddCallerSkip(1))
	}
	return counter{
		logger:      logger,
		timeLogRate: timeLogRate,
		start:       time.Now(),
	}
}

func (c *counter) log(byteSize uint64, completedAddrs int) {
	c.size += byteSize
	c.completedAddrs += uint64(completedAddrs)

	// to keep the floats with only 2 digits
	const cent = 100

	now := time.Now()
	elapsed := now.Sub(c.start).Seconds()
	if elapsed > c.timeLogRate.Seconds() {
		mbs := float64(c.size) / float64(db.Megabyte)
		c.logger.Info(
			"write speed",
			zap.Float64("MB", math.Round(mbs*cent)/cent),
			zap.Float64("MB/s", math.Round(mbs/elapsed*cent)/cent),
			zap.Uint64("completedContracts", c.completedAddrs),
			zap.Float64("completedContracts/s", float64(c.completedAddrs)/elapsed),
			zap.Float64("time", elapsed),
		)
		c.start = now
		c.size = 0
		c.completedAddrs = 0
	}
}
