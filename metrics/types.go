package metrics

import (
	"time"

	"github.com/NethermindEth/juno/metrics/base"
)

type noopCounter struct{}

func (counter noopCounter) Inc()        {}
func (counter noopCounter) Add(float64) {}

func (counter noopCounter) WithLabelValues(lvls ...string) base.Counter { return noopCounter{} }

type noopGauge struct{}

func (counter noopGauge) Set(float64)                               {}
func (counter noopGauge) Inc()                                      {}
func (counter noopGauge) Dec()                                      {}
func (counter noopGauge) Add(float64)                               {}
func (counter noopGauge) Sub(float64)                               {}
func (counter noopGauge) WithLabelValues(lvls ...string) base.Gauge { return noopGauge{} }

type noopHistogram struct{}

func (counter noopHistogram) Observe(float64) {}
func (counter noopHistogram) WithLabelValues(lvls ...string) base.Histogram {
	return noopHistogram{}
}

type noopSummary struct{}

func (counter noopSummary) Observe(float64)                             {}
func (counter noopSummary) WithLabelValues(lvls ...string) base.Summary { return noopHistogram{} }

type noopTimer struct{}

func (counter noopTimer) ObserveDuration() time.Duration { return 0 }
