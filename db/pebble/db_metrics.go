package pebble

import metrics "github.com/NethermindEth/juno/metrics/base"

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
