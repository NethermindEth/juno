package headstate

import (
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

type counter struct {
	logger      log.StructuredLogger
	timeLogRate time.Duration
	start       time.Time
	size        uint64
	addrCount   uint64
}

func newCounter(logger log.StructuredLogger, timeLogRate time.Duration) counter {
	return counter{
		logger:      logger,
		timeLogRate: timeLogRate,
		start:       time.Now(),
	}
}

func (c *counter) log(byteSize uint64, addrCount int) {
	c.size += byteSize
	c.addrCount += uint64(addrCount)

	now := time.Now()
	elapsed := now.Sub(c.start).Seconds()
	if elapsed > float64(c.timeLogRate.Seconds()) {
		mbs := float64(c.size) / float64(db.Megabyte)
		c.logger.Info(
			"write speed",
			zap.Float64("MB", mbs),
			zap.Float64("MB/s", mbs/elapsed),
			zap.Uint64("contracts", c.addrCount),
			zap.Float64("contracts/s", float64(c.addrCount)/elapsed),
			zap.Float64("time", elapsed),
		)
		c.start = now
		c.size = 0
		c.addrCount = 0
	}
}
