package trie

import (
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/performance"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	allReads                     uint64
	allReadsTime                 int64
	allCommitHashCalculationTime int64
)

func incCounter(c *uint64) {
	atomic.AddUint64(c, 1)
}

func addDuration(c *int64, d time.Duration) {
	atomic.AddInt64(c, d.Nanoseconds())
	performance.AddDuration(getTrieMetricName(c), d.Nanoseconds())
}

func getTrieMetricName(c *int64) string {
	switch c {
	case &allCommitHashCalculationTime:
		return "allCommitHashCalculationTime"
	default:
		return "unknown"
	}
}

type DeprecatedTrieMetricsCollector struct{}

func (c *DeprecatedTrieMetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	descs := []*prometheus.Desc{
		prometheus.NewDesc("x_trie_all_reads", "All trie node reads", nil, nil),
		prometheus.NewDesc("x_trie_all_reads_time_ns", "Total time spent reading nodes (ns)", nil, nil),
		prometheus.NewDesc("x_trie_all_commit_hash_calculation_time_ns", "Total time spent calculating hash (ns)", nil, nil),
	}
	for _, d := range descs {
		ch <- d
	}
}

func (c *DeprecatedTrieMetricsCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_trie_all_reads", "All trie node reads", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadUint64(&allReads)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_trie_all_reads_time_ns", "Total time spent reading nodes (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allReadsTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_trie_all_commit_hash_calculation_time_ns", "Total time spent calculating hash (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allCommitHashCalculationTime)),
	)
}
