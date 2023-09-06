package metrics

import (
	"time"
)

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

type Observer interface {
	Observe(float64)
}

type Timer interface {
	ObserveDuration() time.Duration
}

type Opts struct {
	Namespace string
	Subsystem string
	Name      string
}

type SummaryOpts Opts
type CounterOpts Opts
type GaugeOpts Opts
type HistogramOpts struct {
	Namespace string
	Subsystem string
	Name      string

	Buckets []float64
}

var (
	enabled bool
)

func Enable() {
	enabled = true
}

// VoidFactory returns metrics factory without any collection.
func VoidFactory() Factory {
	return &noopFactory{}
}
