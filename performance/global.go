package performance

import (
	"sync"
)

var (
	globalReporter *PerformanceReporter
	reporterOnce   sync.Once
)

// SetGlobalReporter sets the global performance reporter
func SetGlobalReporter(reporter *PerformanceReporter) {
	reporterOnce.Do(func() {
		globalReporter = reporter
	})
}

// GetGlobalReporter returns the global performance reporter
func GetGlobalReporter() *PerformanceReporter {
	return globalReporter
}

// AddDuration adds duration to the global reporter
func AddDuration(metric string, duration int64) {
	if globalReporter != nil {
		globalReporter.AddDuration(metric, duration)
	}
}

// UpdateBlockNumber updates block number in the global reporter
func UpdateBlockNumber(blockNumber uint64) {
	if globalReporter != nil {
		globalReporter.UpdateBlockNumber(blockNumber)
	}
}
