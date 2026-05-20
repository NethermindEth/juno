package trie

import (
	"fmt"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

type counter struct {
	logger         log.StructuredLogger
	timeLogRate    time.Duration
	migrationStart time.Time
	allTries       uint64
	allNodes       uint64
	totalTries     uint64
	totalNodes     uint64
	start          time.Time
	size           uint64
	tries          uint64
	nodes          uint64
}

func newCounter(
	logger log.StructuredLogger,
	timeLogRate time.Duration,
	allTries, allNodes uint64,
) counter {
	now := time.Now()
	return counter{
		logger:         logger,
		timeLogRate:    timeLogRate,
		migrationStart: now,
		start:          now,
		allTries:       allTries,
		allNodes:       allNodes,
	}
}

func (c *counter) log(byteSize uint64, tries, nodes int) {
	c.size += byteSize
	c.tries += uint64(tries)
	c.nodes += uint64(nodes)
	c.totalTries += uint64(tries)
	c.totalNodes += uint64(nodes)

	now := time.Now()
	elapsed := now.Sub(c.start).Seconds()
	if elapsed > c.timeLogRate.Seconds() {
		mbs := float64(c.size) / float64(db.Megabyte)
		c.logger.Info(
			"write speed",
			zap.Float64("MB", mbs),
			zap.Float64("MB/s", mbs/elapsed),
			zap.Float64("nodes/s", float64(c.nodes)/elapsed),
			zap.Float64("tries/s", float64(c.tries)/elapsed),
			zap.String("tries_processed", fmtPercent(c.totalTries, c.allTries)),
			zap.String("nodes_processed", fmtPercent(c.totalNodes, c.allNodes)),
			zap.Float64("totalTime", now.Sub(c.migrationStart).Seconds()),
		)
		c.start = now
		c.size = 0
		c.tries = 0
		c.nodes = 0
	}
}

func fmtPercent(done, total uint64) string {
	if total == 0 {
		return "100.0%"
	}
	return fmt.Sprintf("%.1f%%", 100.0*float64(done)/float64(total))
}
