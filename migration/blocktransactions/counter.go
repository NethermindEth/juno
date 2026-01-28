package blocktransactions

import (
	"time"

	"github.com/NethermindEth/juno/utils"
	"go.uber.org/zap"
)

type counter struct {
	logger      utils.StructuredLogger
	timeLogRate time.Duration
	start       time.Time
	size        uint64
	txCount     uint64
	blockCount  uint64
}

func newCounter(logger utils.StructuredLogger, timeLogRate time.Duration) counter {
	return counter{
		logger:      logger,
		timeLogRate: timeLogRate,
		start:       time.Now(),
		size:        0,
		txCount:     0,
	}
}

func (c *counter) log(byteSize uint64, txCount, blockCount int) {
	c.size += byteSize
	c.txCount += uint64(txCount)
	c.blockCount += uint64(blockCount)

	now := time.Now()
	elapsed := now.Sub(c.start).Seconds()
	if elapsed > float64(timeLogRate.Seconds()) {
		mbs := float64(c.size) / float64(utils.Megabyte)
		c.logger.Info(
			"write speed",
			zap.Float64("MB", mbs),
			zap.Float64("MB/s", mbs/elapsed),
			zap.Uint64("txs", c.txCount),
			zap.Float64("txs/s", float64(c.txCount)/elapsed),
			zap.Uint64("blocks", c.blockCount),
			zap.Float64("blocks/s", float64(c.blockCount)/elapsed),
			zap.Float64("time", elapsed),
		)
		c.start = now
		c.size = 0
		c.txCount = 0
		c.blockCount = 0
	}
}
