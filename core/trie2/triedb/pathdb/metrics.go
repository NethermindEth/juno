package pathdb

import (
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	diffLayerHits  uint64
	cleanCacheHits uint64
	dirtyCacheHits uint64
	diskReads      uint64
	allReads       uint64
	allReadsTime   int64
)

func incCounter(c *uint64) {
	atomic.AddUint64(c, 1)
}

func addDuration(d time.Duration) {
	atomic.AddInt64(&allReadsTime, d.Nanoseconds())
}

type TrieMetricsCollector struct{}

func (c *TrieMetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	// One-time descriptions
	descs := []*prometheus.Desc{
		prometheus.NewDesc("x_trie2_diff_layer_hits", "DiffLayer cache hits", nil, nil),
		prometheus.NewDesc("x_trie2_clean_cache_hits", "Clean cache hits", nil, nil),
		prometheus.NewDesc("x_trie2_dirty_cache_hits", "Dirty cache hits", nil, nil),
		prometheus.NewDesc("x_trie2_disk_reads", "Disk reads", nil, nil),
		prometheus.NewDesc("x_trie2_all_reads", "All trie node reads", nil, nil),
		prometheus.NewDesc("x_trie2_all_reads_time_ns", "Total time spent reading nodes (ns)", nil, nil),
	}
	for _, d := range descs {
		ch <- d
	}
}

func (c *TrieMetricsCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_trie2_diff_layer_hits", "DiffLayer cache hits", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadUint64(&diffLayerHits)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_trie2_clean_cache_hits", "Clean cache hits", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadUint64(&cleanCacheHits)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_trie2_dirty_cache_hits", "Dirty cache hits", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadUint64(&dirtyCacheHits)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_trie2_disk_reads", "Disk reads", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadUint64(&diskReads)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_trie2_all_reads", "All trie node reads", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadUint64(&allReads)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_trie2_all_reads_time_ns", "Total time spent reading nodes (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allReadsTime)),
	)
}
