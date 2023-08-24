package prometheus

import "github.com/prometheus/client_golang/prometheus"

var defaultRegistry = NewRegistry()

func NewRegistry() *prometheus.Registry {
	registry := prometheus.NewRegistry()
	// TODO Should register prometheus collectors such as goCollector etc?
	return registry
}
