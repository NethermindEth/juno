package jsonrpc

import "github.com/NethermindEth/juno/metrics"

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
