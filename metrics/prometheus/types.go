package prometheus

import (
	"github.com/NethermindEth/juno/metrics/base"
	"github.com/prometheus/client_golang/prometheus"
)

type counterVec struct {
	v *prometheus.CounterVec
}

func (c counterVec) WithLabelValues(lvls ...string) base.Counter {
	return c.v.WithLabelValues(lvls...)
}

type gaugeVec struct {
	v *prometheus.GaugeVec
}

func (c gaugeVec) WithLabelValues(lvls ...string) base.Gauge {
	return c.v.WithLabelValues(lvls...)
}

type histogramVec struct {
	v *prometheus.HistogramVec
}

func (c histogramVec) WithLabelValues(lvls ...string) base.Histogram {
	return c.v.WithLabelValues(lvls...)
}

type summaryVec struct {
	v *prometheus.SummaryVec
}

func (c summaryVec) WithLabelValues(lvls ...string) base.Summary {
	return c.v.WithLabelValues(lvls...)
}
