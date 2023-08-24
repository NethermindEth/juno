package jsonrpc

import metrics "github.com/NethermindEth/juno/metrics/base"

type websocketReporter struct {
	requests metrics.Counter
}

func newWebsocketReporter(factory metrics.Factory) websocketReporter {
	return websocketReporter{
		requests: factory.NewCounter(metrics.CounterOpts{
			Namespace: "rpc",
			Subsystem: "ws",
			Name:      "requests",
		}),
	}
}
