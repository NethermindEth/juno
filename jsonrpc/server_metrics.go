package jsonrpc

import "github.com/NethermindEth/juno/metrics"

type serverReporter struct {
	requests metrics.Vec[metrics.Counter]
}

func newServerReporter(factory metrics.Factory) serverReporter {
	return serverReporter{
		requests: factory.NewCounterVec(metrics.CounterOpts{
			Namespace: "rpc",
			Subsystem: "server",
			Name:      "requests",
		}, []string{"method"}),
	}
}
