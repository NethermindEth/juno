package prometheus

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func Handler(registry *prometheus.Registry) http.Handler {
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}
