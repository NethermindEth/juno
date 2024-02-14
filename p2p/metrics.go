package p2p

import (
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/prometheus/client_golang/prometheus"
)

func runMetrics(store peerstore.Peerstore) {
	numberOfPeersGauge := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "p2p",
		Name:      "number_of_peers",
	}, func() float64 {
		peersLen := store.Peers().Len()
		return float64(peersLen)
	})

	prometheus.MustRegister(numberOfPeersGauge)
}
