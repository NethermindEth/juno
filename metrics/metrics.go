package metrics

import (
	"errors"
	"net/http"

	"github.com/NethermindEth/juno/metrics/base"
	junoPrometheus "github.com/NethermindEth/juno/metrics/prometheus"
)

var (
	enabled                bool
	errUnsupportedRegistry = errors.New("unsupported registry")
)

func Enabled() bool {
	return enabled
}

func Enable() {
	enabled = true
}

func PrometheusRegistry() base.Registry {
	return &promRegistryWrapper{registry: junoPrometheus.NewRegistry()}
}

// PrometheusFactory excepts registry to be created with PrometheusRegistry function
func PrometheusFactory(registry base.Registry) (base.Factory, error) {
	if !Enabled() {
		return &noopFactory{}, nil
	}
	switch v := registry.(type) {
	case *promRegistryWrapper:
		return junoPrometheus.Factory(v.registry), nil
	default:
		return nil, errUnsupportedRegistry
	}
}

// PrometheusHandler excepts registry to be created with PrometheusRegistry function
func PrometheusHandler(registry base.Registry) (http.Handler, error) {
	if !Enabled() {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), nil
	}
	switch v := registry.(type) {
	case *promRegistryWrapper:
		return junoPrometheus.Handler(v.registry), nil
	default:
		return nil, errUnsupportedRegistry
	}
}

// VoidFactory returns metrics factory without any collection.
func VoidFactory() base.Factory {
	return &noopFactory{}
}
