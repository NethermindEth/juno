package trie

import (
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Global counters (package-level, not passed down)
var (
	allReads     uint64
	allReadsTime int64
)

func incCounter(c *uint64) {
	atomic.AddUint64(c, 1)
}

func addDuration(d time.Duration) {
	atomic.AddInt64(&allReadsTime, d.Nanoseconds())
}

type DeprecatedTrieMetricsCollector struct{}

func (c *DeprecatedTrieMetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	// One-time descriptions
	descs := []*prometheus.Desc{
		prometheus.NewDesc("trie_all_reads", "All trie node reads", nil, nil),
		prometheus.NewDesc("trie_all_reads_time_ns", "Total time spent reading nodes (ns)", nil, nil),
	}
	for _, d := range descs {
		ch <- d
	}
}

func (c *DeprecatedTrieMetricsCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("trie_all_reads", "All trie node reads", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadUint64(&allReads)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("trie_all_reads_time_ns", "Total time spent reading nodes (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allReadsTime)),
	)
}
