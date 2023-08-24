package base

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Registry interface {
	MustRegister(cs ...Collector)
	Register(c Collector) error
	Unregister(c Collector) bool
}

type SummaryOpts prometheus.SummaryOpts
type CounterOpts prometheus.CounterOpts
type GaugeOpts prometheus.GaugeOpts
type HistogramOpts prometheus.HistogramOpts

type Factory interface {
	NewCounter(opts CounterOpts) Counter
	NewCounterVec(opts CounterOpts, labelNames []string) Vec[Counter]
	NewGauge(opts GaugeOpts) Gauge
	NewGaugeVec(opts GaugeOpts, labelNames []string) Vec[Gauge]
	NewHistogram(opts HistogramOpts) Histogram
	NewHistogramVec(opts HistogramOpts, labelNames []string) Vec[Histogram]
	NewSummary(opts SummaryOpts) Summary
	NewSummaryVec(opts SummaryOpts, labelNames []string) Vec[Summary]
	NewTimer(o Observer) Timer
}

type Summary interface {
	Observe(float64)
}

type Histogram interface {
	Observe(float64)
}

type Gauge interface {
	Set(float64)
	Inc()
	Dec()
	Add(float64)
	Sub(float64)
}

type Vec[T any] interface {
	WithLabelValues(lvs ...string) T
}

type Counter interface {
	Inc()
	Add(float64)
}

type Observer prometheus.Observer
type Collector prometheus.Collector

type Timer interface {
	ObserveDuration() time.Duration
}
