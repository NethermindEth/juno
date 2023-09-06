package pebble

import "github.com/NethermindEth/juno/metrics"

type dbReporter struct {
	writeCounter metrics.Counter
	readCounter  metrics.Counter
}

func newSyncReporter(factory metrics.Factory) *dbReporter {
	return &dbReporter{
		writeCounter: factory.NewCounter(metrics.CounterOpts{
			Namespace: "db",
			Name:      "write",
		}),
		readCounter: factory.NewCounter(metrics.CounterOpts{
			Namespace: "db",
			Name:      "read",
		}),
	}
}
