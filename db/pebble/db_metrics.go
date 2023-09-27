package pebble

import (
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/cockroachdb/pebble"
)

// metricsReporter is an interface designed for exporting Pebble database metrics.
type metricsReporter interface {
	// report collects and exposes important performance statistics like reads, writes and other relevant information.
	report(stats *pebble.Metrics)
	setListener(l db.EventListener)
}

// meter gathers metrics based on provided interval and reports them.
func (d *DB) meter(interval time.Duration) {
	timer := time.NewTimer(interval)
	defer timer.Stop()

	// Gather metrics continuously at specified interval.
	for {
		stats := d.pebble.Metrics()
		for _, reporter := range d.reporters {
			reporter.report(stats)
		}

		<-timer.C
		timer.Reset(interval)
	}
}

// levelsReporter reports about `pebble.Metrics.Levels` metrics.
type levelsReporter struct {
	// previous data
	cache struct {
		Lvls [7]pebble.LevelMetrics // pebble has only 7 lvls
	}

	ready    atomic.Bool
	listener db.LevelsListener
}

func (reporter *levelsReporter) setListener(l db.EventListener) {
	reporter.listener = l
	reporter.ready.Store(true)
}

func (reporter *levelsReporter) report(stats *pebble.Metrics) {
	if !reporter.ready.Load() {
		return
	}
	// swap cache
	levels := stats.Levels
	cache := reporter.cache
	reporter.cache.Lvls = levels

	formatted := db.LevelsMetrics{}
	for i := 0; i < len(stats.Levels) && i < len(formatted.Level); i++ {
		formatted.Level[i].NumFiles = levels[i].NumFiles
		formatted.Level[i].Size = levels[i].Size
		formatted.Level[i].Score = levels[i].Score
		// These metrics are only ever increasing, return delta instead of total.
		formatted.Level[i].BytesIn = levels[i].BytesIn - cache.Lvls[i].BytesIn
		formatted.Level[i].BytesIngested = levels[i].BytesIngested - cache.Lvls[i].BytesIngested
		formatted.Level[i].BytesMoved = levels[i].BytesMoved - cache.Lvls[i].BytesMoved
		formatted.Level[i].BytesRead = levels[i].BytesRead - cache.Lvls[i].BytesRead
		formatted.Level[i].BytesCompacted = levels[i].BytesCompacted - cache.Lvls[i].BytesCompacted
		formatted.Level[i].BytesFlushed = levels[i].BytesFlushed - cache.Lvls[i].BytesFlushed
		formatted.Level[i].TablesCompacted = levels[i].TablesCompacted - cache.Lvls[i].TablesCompacted
		formatted.Level[i].TablesFlushed = levels[i].TablesFlushed - cache.Lvls[i].TablesFlushed
		formatted.Level[i].TablesIngested = levels[i].TablesIngested - cache.Lvls[i].TablesIngested
		formatted.Level[i].TablesMoved = levels[i].TablesMoved - cache.Lvls[i].TablesMoved
	}
	reporter.listener.OnLevels(formatted)
}
