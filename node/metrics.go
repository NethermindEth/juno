package node

import (
	"context"
	"math"
	"strconv"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/clients/gateway"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jemalloc"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/l1"
	"github.com/NethermindEth/juno/sync"
	"github.com/cockroachdb/pebble"
	"github.com/prometheus/client_golang/prometheus"
)

const l1MetricsTimeout = 5 * time.Second

func makeDBMetrics() db.EventListener {
	latencyBuckets := []float64{
		25,
		50,
		75,
		100,
		250,
		500,
		1000, // 1ms
		2000,
		3000,
		4000,
		5000,
		10000,
		50000,
		500000,
		math.Inf(0),
	}
	readLatencyHistogram := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "db",
		Name:      "read_latency",
		Help:      "Database read operation latency in microseconds",
		Buckets:   latencyBuckets,
	})
	writeLatencyHistogram := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "db",
		Name:      "write_latency",
		Help:      "Database write operation latency in microseconds",
		Buckets:   latencyBuckets,
	})
	commitLatency := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "db",
		Name:      "commit_latency",
		Help:      "Database transaction commit latency in microseconds",
		Buckets: []float64{
			5000,
			10000,
			20000,
			30000,
			40000,
			50000,
			100000, // 100ms
			200000,
			300000,
			500000,
			1000000,
			math.Inf(0),
		},
	})

	prometheus.MustRegister(readLatencyHistogram, writeLatencyHistogram, commitLatency)
	return &db.SelectiveListener{
		OnIOCb: func(write bool, duration time.Duration) {
			if write {
				writeLatencyHistogram.Observe(float64(duration.Microseconds()))
			} else {
				readLatencyHistogram.Observe(float64(duration.Microseconds()))
			}
		},
		OnCommitCb: func(duration time.Duration) {
			commitLatency.Observe(float64(duration.Microseconds()))
		},
	}
}

func makeHTTPMetrics() jsonrpc.NewRequestListener {
	reqCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "rpc",
		Subsystem: "http",
		Name:      "requests",
		Help:      "Total number of HTTP RPC requests received",
	})
	prometheus.MustRegister(reqCounter)

	return &jsonrpc.SelectiveListener{
		OnNewRequestCb: func(method string) {
			reqCounter.Inc()
		},
	}
}

func makeWSMetrics() jsonrpc.NewRequestListener {
	reqCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "rpc",
		Subsystem: "ws",
		Name:      "requests",
		Help:      "Total number of WebSocket RPC requests received",
	})
	prometheus.MustRegister(reqCounter)

	return &jsonrpc.SelectiveListener{
		OnNewRequestCb: func(method string) {
			reqCounter.Inc()
		},
	}
}

func makeRPCMetrics(versions ...string) []jsonrpc.EventListener {
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "rpc",
		Subsystem: "server",
		Name:      "requests",
		Help:      "Total number of RPC requests by method and version",
	}, []string{"method", "version"})
	failedRequests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "rpc",
		Subsystem: "server",
		Name:      "failed_requests",
		Help:      "Total number of failed RPC requests by method and version",
	}, []string{"method", "version"})
	requestLatencies := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "rpc",
		Subsystem: "server",
		Name:      "requests_latency",
		Help:      "RPC request processing latency in seconds",
	}, []string{"method", "version"})
	prometheus.MustRegister(requests, failedRequests, requestLatencies)

	listeners := make([]jsonrpc.EventListener, len(versions))
	for i, version := range versions {
		version := version
		listeners[i] = &jsonrpc.SelectiveListener{
			OnNewRequestCb: func(method string) {
				requests.WithLabelValues(method, version).Inc()
			},
			OnRequestHandledCb: func(method string, took time.Duration) {
				requestLatencies.WithLabelValues(method, version).Observe(took.Seconds())
			},
			OnRequestFailedCb: func(method string, data any) {
				failedRequests.WithLabelValues(method, version).Inc()
			},
		}
	}
	return listeners
}

func makeSyncMetrics(syncReader sync.Reader, bcReader blockchain.Reader) sync.EventListener {
	opTimerHistogram := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "sync",
		Name:      "timers",
		Help:      "Synchronisation operation duration in seconds",
	}, []string{"op"})
	blockCount := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "sync",
		Name:      "blocks",
		Help:      "Total number of blocks processed during Synchronisation",
	})
	reorgCount := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "sync",
		Name:      "reorganisations",
		Help:      "Total number of blockchain reorganisations encountered",
	})
	chainHeightGauge := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "sync",
		Name:      "blockchain_height",
		Help:      "Current blockchain height (latest block number)",
	}, func() float64 {
		height, _ := bcReader.Height()
		return float64(height)
	})
	bestBlockGauge := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "sync",
		Name:      "best_known_block_number",
		Help:      "Highest known block number from the network",
	}, func() float64 {
		bestHeader := syncReader.HighestBlockHeader()
		if bestHeader != nil {
			return float64(bestHeader.Number)
		}
		return 0
	})

	prometheus.MustRegister(opTimerHistogram, blockCount, chainHeightGauge, bestBlockGauge, reorgCount)

	return &sync.SelectiveListener{
		OnSyncStepDoneCb: func(op string, blockNum uint64, took time.Duration) {
			opTimerHistogram.WithLabelValues(op).Observe(took.Seconds())
			if op == sync.OpStore {
				blockCount.Inc()
			}
		},
		OnReorgCb: func(blockNum uint64) {
			reorgCount.Inc()
		},
	}
}

func makeJunoMetrics(version string) {
	prometheus.MustRegister(prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   "juno",
		Name:        "info",
		Help:        "Information about the Juno binary",
		ConstLabels: prometheus.Labels{"version": version},
	}))
}

func makeBlockchainMetrics() blockchain.EventListener {
	reads := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "blockchain",
		Name:      "reads",
		Help:      "Total number of calls to 'blockchain.Blockchain' read methods",
	}, []string{"method"})
	prometheus.MustRegister(reads)

	return &blockchain.SelectiveListener{
		OnReadCb: func(method string) {
			reads.WithLabelValues(method).Inc()
		},
	}
}

func makeL1Metrics(bcReader blockchain.Reader, l1Subscriber l1.Subscriber) l1.EventListener {
	l2BlockFinalizedOnL1 := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "l1",
		Name:      "l2_finalised_height",
		Help:      "Latest L2 block number that has been finalised on L1",
	}, func() float64 {
		l1Head, err := bcReader.L1Head()
		if err != nil {
			return 0
		}
		return float64(l1Head.BlockNumber)
	})
	prometheus.MustRegister(l2BlockFinalizedOnL1)

	l1Height := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "l1",
		Name:      "finalised_height",
		Help:      "Current L1 (Ethereum) finalised blockchain height",
	}, func() float64 {
		ctx, cancel := context.WithTimeout(context.Background(), l1MetricsTimeout)
		defer cancel()
		height, err := l1Subscriber.FinalisedHeight(ctx)
		if err != nil {
			return 0
		}
		return float64(height)
	})
	prometheus.MustRegister(l1Height)

	l1LatestHeight := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "l1",
		Name:      "latest_height",
		Help:      "Current latest L1 (Ethereum) blockchain height",
	}, func() float64 {
		ctx, cancel := context.WithTimeout(context.Background(), l1MetricsTimeout)
		defer cancel()
		height, err := l1Subscriber.LatestHeight(ctx)
		if err != nil {
			return 0
		}
		return float64(height)
	})
	prometheus.MustRegister(l1LatestHeight)

	requestLatencies := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "l1",
		Subsystem: "client",
		Name:      "request_latency",
		Help:      "L1 client request latency in seconds",
	}, []string{"method"})
	prometheus.MustRegister(requestLatencies)

	return l1.SelectiveListener{
		OnL1CallCb: func(method string, took time.Duration) {
			requestLatencies.WithLabelValues(method).Observe(took.Seconds())
		},
	}
}

func makeFeederMetrics() feeder.EventListener {
	requestLatencies := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "feeder",
		Subsystem: "client",
		Name:      "request_latency",
		Help:      "Feeder client request latency in seconds",
	}, []string{"method", "status"})
	prometheus.MustRegister(requestLatencies)
	return &feeder.SelectiveListener{
		OnResponseCb: func(urlPath string, status int, took time.Duration) {
			statusString := strconv.FormatInt(int64(status), 10)
			requestLatencies.WithLabelValues(urlPath, statusString).Observe(took.Seconds())
		},
	}
}

func makeGatewayMetrics() gateway.EventListener {
	requestLatencies := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "gateway",
		Subsystem: "client",
		Name:      "request_latency",
		Help:      "Gateway client request latency in seconds",
	}, []string{"method", "status"})
	prometheus.MustRegister(requestLatencies)
	return &gateway.SelectiveListener{
		OnResponseCb: func(urlPath string, status int, took time.Duration) {
			statusString := strconv.FormatInt(int64(status), 10)
			requestLatencies.WithLabelValues(urlPath, statusString).Observe(took.Seconds())
		},
	}
}

func makePebbleMetrics(nodeDB db.KeyValueStore) {
	pebbleDB, ok := nodeDB.Impl().(*pebble.DB)
	if !ok {
		return
	}

	blockCacheSize := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "pebble",
		Subsystem: "block_cache",
		Name:      "size",
		Help:      "Current size of Pebble block cache in bytes",
	}, func() float64 {
		return float64(pebbleDB.Metrics().BlockCache.Size)
	})
	blockHitRate := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "pebble",
		Subsystem: "block_cache",
		Name:      "hit_rate",
		Help:      "Hit rate of Pebble block cache (hits / (hits + misses))",
	}, func() float64 {
		metrics := pebbleDB.Metrics()
		return float64(metrics.BlockCache.Hits) / float64(metrics.BlockCache.Hits+metrics.BlockCache.Misses)
	})
	tableCacheSize := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "pebble",
		Subsystem: "table_cache",
		Name:      "size",
		Help:      "Current size of Pebble table cache in bytes",
	}, func() float64 {
		return float64(pebbleDB.Metrics().TableCache.Size)
	})
	tableHitRate := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "pebble",
		Subsystem: "table_cache",
		Name:      "hit_rate",
		Help:      "Hit rate of Pebble table cache (hits / (hits + misses))",
	}, func() float64 {
		metrics := pebbleDB.Metrics()
		return float64(metrics.TableCache.Hits) / float64(metrics.TableCache.Hits+metrics.TableCache.Misses)
	})
	prometheus.MustRegister(blockCacheSize, blockHitRate, tableCacheSize, tableHitRate)
}

func makeJeMallocMetrics() {
	active := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "jemalloc",
		Name:      "active",
		Help:      "Current active memory usage in bytes",
	}, func() float64 {
		return float64(jemalloc.GetActive())
	})
	prometheus.MustRegister(active)
}

func makeVMThrottlerMetrics(throttledVM *ThrottledVM) {
	vmJobs := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "vm",
		Name:      "jobs",
		Help:      "Number of currently running VM jobs",
	}, func() float64 {
		return float64(throttledVM.JobsRunning())
	})
	vmQueue := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "vm",
		Name:      "queue",
		Help:      "Number of VM jobs waiting in the queue",
	}, func() float64 {
		return float64(throttledVM.QueueLen())
	})
	prometheus.MustRegister(vmJobs, vmQueue)
}
