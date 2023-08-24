package jsonrpc

import metrics "github.com/NethermindEth/juno/metrics/base"

type httpReporter struct {
	requests metrics.Counter
}

func newHttpReporter(factory metrics.Factory) httpReporter {
	return httpReporter{
		requests: factory.NewCounter(metrics.CounterOpts{
			Namespace: "rpc",
			Subsystem: "http",
			Name:      "requests",
		}),
	}
}
