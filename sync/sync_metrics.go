package sync

import metrics "github.com/NethermindEth/juno/metrics/base"

type syncReporter struct {
	factory     metrics.Factory
	opTimers    metrics.Vec[metrics.Histogram]
	totalBlocks metrics.Counter
}

func newSyncReporter(factory metrics.Factory) syncReporter {
	return syncReporter{
		factory: factory,
		opTimers: factory.NewHistogramVec(metrics.HistogramOpts{
			Namespace: "sync",
			Name:      "timers",
		}, []string{"op"}),
		totalBlocks: factory.NewCounter(metrics.CounterOpts{
			Namespace: "sync",
			Name:      "blocks",
		}),
	}
}
