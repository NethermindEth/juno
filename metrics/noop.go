package metrics

import (
	"time"
)

type noopFactory struct{}

func (d *noopFactory) NewCounter(opts CounterOpts) Counter {
	return noopCounter{}
}

func (d *noopFactory) NewCounterVec(opts CounterOpts, labelNames []string) Vec[Counter] {
	return noopCounter{}
}

func (d *noopFactory) NewGauge(opts GaugeOpts) Gauge {
	return noopGauge{}
}

func (d *noopFactory) NewGaugeVec(opts GaugeOpts, labelNames []string) Vec[Gauge] {
	return noopGauge{}
}

func (d *noopFactory) NewHistogram(opts HistogramOpts) Histogram {
	return noopHistogram{}
}

func (d *noopFactory) NewHistogramVec(opts HistogramOpts, labelNames []string) Vec[Histogram] {
	return noopHistogram{}
}

func (d *noopFactory) NewSummary(opts SummaryOpts) Summary {
	return noopSummary{}
}

func (d *noopFactory) NewSummaryVec(opts SummaryOpts, labelNames []string) Vec[Summary] {
	return noopSummary{}
}

func (d *noopFactory) NewTimer(o Observer) Timer {
	return noopTimer{}
}

type noopCounter struct{}

func (counter noopCounter) Inc()        {}
func (counter noopCounter) Add(float64) {}
func (counter noopCounter) WithLabelValues(lvls ...string) Counter {
	return noopCounter{}
}

type noopGauge struct{}

func (counter noopGauge) Set(float64) {}
func (counter noopGauge) Inc()        {}
func (counter noopGauge) Dec()        {}
func (counter noopGauge) Add(float64) {}
func (counter noopGauge) Sub(float64) {}
func (counter noopGauge) WithLabelValues(lvls ...string) Gauge {
	return noopGauge{}
}

type noopHistogram struct{}

func (counter noopHistogram) Observe(float64) {}
func (counter noopHistogram) WithLabelValues(lvls ...string) Histogram {
	return noopHistogram{}
}

type noopSummary struct{}

func (counter noopSummary) Observe(float64) {}
func (counter noopSummary) WithLabelValues(lvls ...string) Summary {
	return noopHistogram{}
}

type noopTimer struct{}

func (counter noopTimer) ObserveDuration() time.Duration {
	return 0
}
