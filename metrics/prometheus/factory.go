package prometheus

import (
	"sync"

	"github.com/NethermindEth/juno/metrics/base"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type factory struct {
	mu      sync.Mutex
	factory promauto.Factory
}

// Factory is a wrapper around prometheus factory implementing Factory interface.
// When no registry is provided it uses the default one.
func Factory(registry *prometheus.Registry) base.Factory {
	if registry == nil {
		registry = defaultRegistry
	}
	return &factory{factory: promauto.With(registry)}
}

func (d *factory) NewCounter(opts base.CounterOpts) base.Counter {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.factory.NewCounter(prometheus.CounterOpts(opts))
}

func (d *factory) NewCounterVec(opts base.CounterOpts, labelNames []string) base.Vec[base.Counter] {
	d.mu.Lock()
	defer d.mu.Unlock()
	return counterVec{d.factory.NewCounterVec(prometheus.CounterOpts(opts), labelNames)}
}

func (d *factory) NewGauge(opts base.GaugeOpts) base.Gauge {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.factory.NewGauge(prometheus.GaugeOpts(opts))
}

func (d *factory) NewGaugeVec(opts base.GaugeOpts, labelNames []string) base.Vec[base.Gauge] {
	d.mu.Lock()
	defer d.mu.Unlock()
	return gaugeVec{d.factory.NewGaugeVec(prometheus.GaugeOpts(opts), labelNames)}
}

func (d *factory) NewHistogram(opts base.HistogramOpts) base.Histogram {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.factory.NewHistogram(prometheus.HistogramOpts(opts))
}

func (d *factory) NewHistogramVec(opts base.HistogramOpts, labelNames []string) base.Vec[base.Histogram] {
	d.mu.Lock()
	defer d.mu.Unlock()
	return histogramVec{d.factory.NewHistogramVec(prometheus.HistogramOpts(opts), labelNames)}
}

func (d *factory) NewSummary(opts base.SummaryOpts) base.Summary {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.factory.NewSummary(prometheus.SummaryOpts(opts))
}

func (d *factory) NewSummaryVec(opts base.SummaryOpts, labelNames []string) base.Vec[base.Summary] {
	d.mu.Lock()
	defer d.mu.Unlock()
	return summaryVec{d.factory.NewSummaryVec(prometheus.SummaryOpts(opts), labelNames)}
}

func (d *factory) NewTimer(o base.Observer) base.Timer {
	d.mu.Lock()
	defer d.mu.Unlock()
	return prometheus.NewTimer(o)
}
