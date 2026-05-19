package trie

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
	tries       uint64
}

func newCounter(logger log.StructuredLogger, timeLogRate time.Duration) counter {
	return counter{
		logger:      logger,
		timeLogRate: timeLogRate,
		start:       time.Now(),
	}
}

func (c *counter) log(byteSize uint64, tries int) {
	c.size += byteSize
	c.tries += uint64(tries)

	now := time.Now()
	elapsed := now.Sub(c.start).Seconds()
	if elapsed > float64(c.timeLogRate.Seconds()) {
		mbs := float64(c.size) / float64(db.Megabyte)
		c.logger.Info(
			"write speed",
			zap.Float64("MB", mbs),
			zap.Float64("MB/s", mbs/elapsed),
			zap.Uint64("tries", c.tries),
			zap.Float64("tries/s", float64(c.tries)/elapsed),
			zap.Float64("time", elapsed),
		)
		c.start = now
		c.size = 0
		c.tries = 0
	}
}
