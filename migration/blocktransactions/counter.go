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
	count       uint64
}

func newCounter(logger utils.SimpleLogger, timeLogRate time.Duration) counter {
	return counter{
		logger:      logger,
		timeLogRate: timeLogRate,
		start:       time.Now(),
		size:        0,
		count:       0,
	}
}

func (c *counter) log(byteSize, blockCount uint64) {
	c.size += byteSize
	c.count += blockCount

	now := time.Now()
	elapsed := now.Sub(c.start).Seconds()
	if elapsed > float64(timeLogRate.Seconds()) {
		mbs := float64(c.size) / float64(utils.Megabyte)
		c.logger.Infow(
			"write speed",
			"MB", mbs,
			"MB/s", mbs/elapsed,
			"blocks", c.count,
			"blocks/s", float64(c.count)/elapsed,
			"time", elapsed,
		)
		c.start = now
		c.size = 0
		c.count = 0
	}
}
