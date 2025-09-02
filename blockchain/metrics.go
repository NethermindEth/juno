package blockchain

import (
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	allStore                 uint64
	allStoreTime             int64
	allUpdate                uint64
	allUpdateTime            int64
	allIndexedBatchStoreTime int64
	deprecatedInnerStoreTime int64
	allBatchStoreTime        int64
)

func incCounter(c *uint64) {
	atomic.AddUint64(c, 1)
}

func addDuration(c *int64, d time.Duration) {
	atomic.AddInt64(c, d.Nanoseconds())
}

type BlockchainMetricsCollector struct{}

func (c *BlockchainMetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	// One-time descriptions
	descs := []*prometheus.Desc{
		prometheus.NewDesc("x_blockchain_all_store", "All store", nil, nil),
		prometheus.NewDesc("x_blockchain_all_store_time_ns", "Total time spent storing (ns)", nil, nil),
		prometheus.NewDesc("x_blockchain_all_update", "All update", nil, nil),
		prometheus.NewDesc("x_blockchain_all_update_time_ns", "Total time spent updating (ns)", nil, nil),
		prometheus.NewDesc("x_blockchain_all_indexed_batch_store_time_ns", "Total time spent indexed batch store (ns)", nil, nil),
		prometheus.NewDesc("x_blockchain_all_batch_store_time_ns", "Total time spent batch store (ns)", nil, nil),
		prometheus.NewDesc("x_blockchain_deprecated_inner_store_time_ns", "Total time spent deprecated inner store (ns)", nil, nil),
	}
	for _, d := range descs {
		ch <- d
	}
}

func (c *BlockchainMetricsCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_blockchain_all_store", "All store", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadUint64(&allStore)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_blockchain_all_store_time_ns", "Total time spent storing (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allStoreTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_blockchain_all_update", "All update", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadUint64(&allUpdate)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_blockchain_all_update_time_ns", "Total time spent updating (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allUpdateTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_blockchain_all_indexed_batch_store_time_ns", "Total time spent indexed batch store (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allIndexedBatchStoreTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_blockchain_all_batch_store_time_ns", "Total time spent batch store (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allBatchStoreTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_blockchain_deprecated_inner_store_time_ns", "Total time spent deprecated inner store (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&deprecatedInnerStoreTime)),
	)
}
