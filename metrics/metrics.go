package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var Enabled = false

func MustRegister(collectors ...prometheus.Collector) {
	if Enabled {
		prometheus.MustRegister(collectors...)
	}
}
