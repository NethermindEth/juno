package metrics

import (
	"github.com/NethermindEth/juno/metrics/base"
)

type noopFactory struct{}

func (d *noopFactory) NewCounter(opts base.CounterOpts) base.Counter {
	return noopCounter{}
}

func (d *noopFactory) NewCounterVec(opts base.CounterOpts, labelNames []string) base.Vec[base.Counter] {
	return noopCounter{}
}

func (d *noopFactory) NewGauge(opts base.GaugeOpts) base.Gauge {
	return noopGauge{}
}

func (d *noopFactory) NewGaugeVec(opts base.GaugeOpts, labelNames []string) base.Vec[base.Gauge] {
	return noopGauge{}
}

func (d *noopFactory) NewHistogram(opts base.HistogramOpts) base.Histogram {
	return noopHistogram{}
}

func (d *noopFactory) NewHistogramVec(opts base.HistogramOpts, labelNames []string) base.Vec[base.Histogram] {
	return noopHistogram{}
}

func (d *noopFactory) NewSummary(opts base.SummaryOpts) base.Summary {
	return noopSummary{}
}

func (d *noopFactory) NewSummaryVec(opts base.SummaryOpts, labelNames []string) base.Vec[base.Summary] {
	return noopSummary{}
}

func (d *noopFactory) NewTimer(o base.Observer) base.Timer {
	return noopTimer{}
}
