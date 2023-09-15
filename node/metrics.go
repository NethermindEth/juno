package node

import (
	"github.com/NethermindEth/juno/db"
	"github.com/prometheus/client_golang/prometheus"
)

func makeDBMetrics() db.EventListener {
	readCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "db",
		Name:      "read",
	})
	writeCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "db",
		Name:      "write",
	})
	prometheus.MustRegister(readCounter, writeCounter)

	return &db.SelectiveListener{
		OnIOCb: func(write bool) {
			if write {
				writeCounter.Inc()
			} else {
				readCounter.Inc()
			}
		},
	}
}
