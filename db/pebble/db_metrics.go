package pebble

import (
	"time"

	"github.com/NethermindEth/juno/db"
)

// eventCollector is an interface for writing event-derived metrics.
type eventCollector interface {
	// collect writes it's own metrics into the provided structure.
	write(stats *db.PebbleMetrics)
}

// meter continuously gathers metrics at the specified interval and reports them to the underlying listener.
func (d *DB) meter(interval time.Duration) {
	timer := time.NewTimer(interval)
	defer timer.Stop()

	// Gather metrics continuously at specified interval.
	for {
		// Retrieve Pebble metrics.
		metrics := &db.PebbleMetrics{Src: d.pebble.Metrics()}
		// Derive additional metrics from collected events.
		for _, collector := range d.collectors {
			collector.write(metrics)
		}
		// Notify the listener.
		d.listener.OnPebbleMetrics(metrics)

		<-timer.C
		timer.Reset(interval)
	}
}
