package trie2

import (
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/performance"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	allCommitHashCalculationTime int64
	allCollectTime               int64
)

func addDuration(c *int64, d time.Duration) {
	atomic.AddInt64(c, d.Nanoseconds())
	// Also add to performance reporter
	performance.AddDuration(getTrie2MetricName(c), d.Nanoseconds())
}

func getTrie2MetricName(c *int64) string {
	switch c {
	case &allCommitHashCalculationTime:
		return "allCommitHashCalculationTime"
	case &allCollectTime:
		return "allCollectTime"
	default:
		return "unknown"
	}
}

type TrieMetricsCollector struct{}

func (c *TrieMetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	descs := []*prometheus.Desc{
		prometheus.NewDesc("x_trie2_all_commit_hash_calculation_time_ns", "Total time spent calculating hash (ns)", nil, nil),
		prometheus.NewDesc("x_trie2_all_collect_time_ns", "Total time spent collecting (ns)", nil, nil),
	}
	for _, d := range descs {
		ch <- d
	}
}

func (c *TrieMetricsCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_trie2_all_commit_hash_calculation_time_ns", "Total time spent calculating hash (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allCommitHashCalculationTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_trie2_all_collect_time_ns", "Total time spent collecting (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allCollectTime)),
	)
}
