package blocktransactions

import (
	"time"

	"github.com/NethermindEth/juno/utils"
)

type counter struct {
	logger      utils.SimpleLogger
	timeLogRate time.Duration
	start       time.Time
	size        uint64
	txCount     uint64
	blockCount  uint64
}

func newCounter(logger utils.SimpleLogger, timeLogRate time.Duration) counter {
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
		c.logger.Infow(
			"write speed",
			"MB", mbs,
			"MB/s", mbs/elapsed,
			"txs", c.txCount,
			"txs/s", float64(c.txCount)/elapsed,
			"blocks", c.blockCount,
			"blocks/s", float64(c.blockCount)/elapsed,
			"time", elapsed,
		)
		c.start = now
		c.size = 0
		c.txCount = 0
		c.blockCount = 0
	}
}
