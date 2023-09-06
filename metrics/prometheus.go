package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	defaultPromRegistry = PrometheusRegistry()
)

// PrometheusHandler returns `http.Handler`
// If no registry has been provided, the default one will be used.
func PrometheusHandler(registry *prometheus.Registry) http.Handler {
	if registry == nil {
		registry = defaultPromRegistry
	}
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}

// PrometheusRegistry returns prometheus registry
func PrometheusRegistry() *prometheus.Registry {
	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewBuildInfoCollector())
	registry.MustRegister(collectors.NewGoCollector())
	return registry
}

// PrometheusFactory returns factory that operates based on prometheus types
// If none registry has been provided, the default one will be used.
func PrometheusFactory(registry *prometheus.Registry) Factory {
	if !enabled {
		return &noopFactory{}
	}
	if registry == nil {
		registry = defaultPromRegistry
	}
	return _prometheusFactory(registry)
}

func _prometheusFactory(registry *prometheus.Registry) *prometheusFactory {
	return &prometheusFactory{factory: promauto.With(registry)}
}

// prometheusFactory implements `Factory` interface
type prometheusFactory struct {
	factory promauto.Factory
}

func (d *prometheusFactory) NewCounter(opts CounterOpts) Counter {
	return d.factory.NewCounter(prometheus.CounterOpts{
		Namespace: opts.Namespace,
		Subsystem: opts.Subsystem,
		Name:      opts.Name,
	})
}

func (d *prometheusFactory) NewCounterVec(opts CounterOpts, labelNames []string) Vec[Counter] {
	return promCounterVec{d.factory.NewCounterVec(prometheus.CounterOpts{
		Namespace: opts.Namespace,
		Subsystem: opts.Subsystem,
		Name:      opts.Name,
	}, labelNames)}
}

func (d *prometheusFactory) NewGauge(opts GaugeOpts) Gauge {
	return d.factory.NewGauge(prometheus.GaugeOpts{
		Namespace: opts.Namespace,
		Subsystem: opts.Subsystem,
		Name:      opts.Name,
	})
}

func (d *prometheusFactory) NewGaugeVec(opts GaugeOpts, labelNames []string) Vec[Gauge] {
	return promGaugeVec{d.factory.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: opts.Namespace,
		Subsystem: opts.Subsystem,
		Name:      opts.Name,
	}, labelNames)}
}

func (d *prometheusFactory) NewHistogram(opts HistogramOpts) Histogram {
	return d.factory.NewHistogram(prometheus.HistogramOpts{
		Namespace: opts.Namespace,
		Subsystem: opts.Subsystem,
		Name:      opts.Name,
		Buckets:   opts.Buckets,
	})
}

func (d *prometheusFactory) NewHistogramVec(opts HistogramOpts, labelNames []string) Vec[Histogram] {
	return promHistogram{d.factory.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: opts.Namespace,
		Subsystem: opts.Subsystem,
		Name:      opts.Name,
		Buckets:   opts.Buckets,
	}, labelNames)}
}

func (d *prometheusFactory) NewSummary(opts SummaryOpts) Summary {
	return d.factory.NewSummary(prometheus.SummaryOpts{
		Namespace: opts.Namespace,
		Subsystem: opts.Subsystem,
		Name:      opts.Name,
	})
}

func (d *prometheusFactory) NewSummaryVec(opts SummaryOpts, labelNames []string) Vec[Summary] {
	return promSummaryVec{d.factory.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: opts.Namespace,
		Subsystem: opts.Subsystem,
		Name:      opts.Name,
	}, labelNames)}
}

func (d *prometheusFactory) NewTimer(o Observer) Timer {
	return prometheus.NewTimer(o)
}

type promCounterVec struct {
	v *prometheus.CounterVec
}

func (c promCounterVec) WithLabelValues(lvls ...string) Counter {
	return c.v.WithLabelValues(lvls...)
}

type promGaugeVec struct {
	v *prometheus.GaugeVec
}

func (c promGaugeVec) WithLabelValues(lvls ...string) Gauge {
	return c.v.WithLabelValues(lvls...)
}

type promHistogram struct {
	v *prometheus.HistogramVec
}

func (c promHistogram) WithLabelValues(lvls ...string) Histogram {
	return c.v.WithLabelValues(lvls...)
}

type promSummaryVec struct {
	v *prometheus.SummaryVec
}

func (c promSummaryVec) WithLabelValues(lvls ...string) Summary {
	return c.v.WithLabelValues(lvls...)
}
