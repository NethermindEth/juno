package monitor

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

func New() *Metrics {
	return &Metrics{
		RPCCounterVec: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "rpc_method_calls_total",
			Help: "Number of rpc method calls",
		}, []string{"method", "path"}),
		RPCFailedCounterVec: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "rpc_method_calls_failed_total",
			Help: "Number of failed rpc method calls",
		}, []string{"method", "path"}),
	}
}

type Metrics struct {
	RPCCounterVec       *prometheus.CounterVec
	RPCFailedCounterVec *prometheus.CounterVec
}

func (m *Metrics) RPCCounterInc(method, path string) {
	m.RPCCounterVec.WithLabelValues(method, path).Inc()
}

func (m *Metrics) RPCFailedCounterInc(method, path string) {
	m.RPCFailedCounterVec.WithLabelValues(method, path).Inc()
}
