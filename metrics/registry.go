package metrics

import (
	"github.com/NethermindEth/juno/metrics/base"
	"github.com/prometheus/client_golang/prometheus"
)

// promRegistryWrapper Implements base.Registry
type promRegistryWrapper struct {
	registry *prometheus.Registry
}

func (r *promRegistryWrapper) MustRegister(cs ...base.Collector) {
	converted := make([]prometheus.Collector, len(cs))
	for i := 0; i < len(cs); i++ {
		converted[i] = cs[i]
	}
	r.registry.MustRegister(converted...)
}

func (r *promRegistryWrapper) Register(c base.Collector) error  { return r.registry.Register(c) }
func (r *promRegistryWrapper) Unregister(c base.Collector) bool { return r.registry.Unregister(c) }
