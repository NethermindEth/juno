package node

import (
	"strconv"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/sync"
	"github.com/prometheus/client_golang/prometheus"
)

// pebbleListener listens for pebble metrics.
type pebbleListener struct {
	compTime       prometheus.Counter   // Counter for total time spent in database compaction.
	compRead       prometheus.Gauge     // Counter for the amount of data read during compaction.
	compWrite      prometheus.Gauge     // Counter for the amount of data written during compaction.
	writeDelays    prometheus.Counter   // Counter for the number of write delays due to database compaction.
	writeDelay     prometheus.Counter   // Counter for the duration of write delays due to database compaction.
	diskSize       prometheus.Gauge     // Gauge for the size of all levels in the database.
	diskRead       prometheus.Gauge     // Gauge for the amount of data read from the database.
	diskWrite      prometheus.Gauge     // Gauge for the amount of data written to the database.
	memComp        prometheus.Gauge     // Gauge for the number of memory compactions.
	level0Comp     prometheus.Gauge     // Gauge for the number of table compactions in level 0.
	nonLevel0Comp  prometheus.Gauge     // Gauge for the number of table compactions in non-level 0.
	levelFiles     *prometheus.GaugeVec // Gauge vector for the number of files in each level.
	seekComp       prometheus.Gauge     // Gauge for the number of table compactions caused by read operations.
	manualMemAlloc prometheus.Gauge     // Gauge for the size of non-managed memory allocated.
}

// newPebbleListener creates and returns a new pebbleListener instance.
func newPebbleListener(registry prometheus.Registerer) *pebbleListener {
	const (
		namespace = "db"
		subsystem = "pebble"
	)
	listener := &pebbleListener{}
	listener.compTime = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "compTime",
	})
	listener.compRead = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "compIn",
	})
	listener.compWrite = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "compOut",
	})
	listener.writeDelays = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "writeDelays",
	})
	listener.writeDelay = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "writeDelay",
	})
	listener.diskSize = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "diskSize",
	})
	listener.diskRead = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "diskRead",
	})
	listener.diskWrite = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "diskWrite",
	})
	listener.memComp = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "memComp",
	})
	listener.level0Comp = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "level0Comp",
	})
	listener.nonLevel0Comp = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "nonLevel0Comp",
	})
	listener.levelFiles = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "levelFiles",
	}, []string{"lvl"})
	listener.seekComp = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "seekComp",
	})
	listener.manualMemAlloc = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "manualMemAlloc",
	})
	registry.MustRegister(
		listener.compTime,
		listener.compRead,
		listener.compWrite,
		listener.writeDelays,
		listener.writeDelay,
		listener.diskSize,
		listener.diskRead,
		listener.diskWrite,
		listener.memComp,
		listener.level0Comp,
		listener.nonLevel0Comp,
		listener.levelFiles,
		listener.seekComp,
		listener.manualMemAlloc,
	)
	return listener
}

// gather updates metrics with data from the PebbleMetrics.
func (l *pebbleListener) gather(m *db.PebbleMetrics) {
	l.compTime.Add(m.CompTime.Seconds())
	l.compRead.Set(float64(m.CompRead))
	l.compWrite.Set(float64(m.CompWrite))

	l.writeDelays.Add(float64(m.WriteDelayN))
	l.writeDelay.Add(m.WriteDelay.Seconds())

	l.diskSize.Set(float64(m.DiskSize))
	l.diskRead.Set(float64(m.DiskRead))
	l.diskWrite.Set(float64(m.DiskWrite))

	l.memComp.Set(float64(m.MemComps))
	l.level0Comp.Set(float64(m.Level0Comp))
	l.nonLevel0Comp.Set(float64(m.NonLevel0Comp))
	for i := 0; i < len(m.LevelFiles); i++ {
		l.levelFiles.WithLabelValues(strconv.Itoa(i)).Set(float64(m.LevelFiles[i]))
	}
	l.seekComp.Set(float64(m.SeekComp))
	l.manualMemAlloc.Set(float64(m.ManualMemAlloc))
}

func makeDBMetrics() db.EventListener {
	readCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "db",
		Name:      "read",
	})
	writeCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "db",
		Name:      "write",
	})
	prometheus.MustRegister(readCounter, writeCounter)
	pebbleListener := newPebbleListener(prometheus.DefaultRegisterer)
	return &db.SelectiveListener{
		OnIOCb: func(write bool) {
			if write {
				writeCounter.Inc()
			} else {
				readCounter.Inc()
			}
		},
		OnPebbleMetricsCb: pebbleListener.gather,
	}
}

func makeHTTPMetrics() jsonrpc.NewRequestListener {
	reqCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "rpc",
		Subsystem: "http",
		Name:      "requests",
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
	})
	prometheus.MustRegister(reqCounter)

	return &jsonrpc.SelectiveListener{
		OnNewRequestCb: func(method string) {
			reqCounter.Inc()
		},
	}
}

func makeRPCMetrics(version, legacyVersion string) (jsonrpc.EventListener, jsonrpc.EventListener) {
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "rpc",
		Subsystem: "server",
		Name:      "requests",
	}, []string{"method", "version"})
	failedRequests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "rpc",
		Subsystem: "server",
		Name:      "failed_requests",
	}, []string{"method", "version"})
	requestLatencies := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "rpc",
		Subsystem: "server",
		Name:      "requests_latency",
	}, []string{"method", "version"})
	prometheus.MustRegister(requests, failedRequests, requestLatencies)

	return &jsonrpc.SelectiveListener{
			OnNewRequestCb: func(method string) {
				requests.WithLabelValues(method, version).Inc()
			},
			OnRequestHandledCb: func(method string, took time.Duration) {
				requestLatencies.WithLabelValues(method, version).Observe(took.Seconds())
			},
			OnRequestFailedCb: func(method string, data any) {
				failedRequests.WithLabelValues(method, version).Inc()
			},
		}, &jsonrpc.SelectiveListener{
			OnNewRequestCb: func(method string) {
				requests.WithLabelValues(method, legacyVersion).Inc()
			},
			OnRequestHandledCb: func(method string, took time.Duration) {
				requestLatencies.WithLabelValues(method, legacyVersion).Observe(took.Seconds())
			},
			OnRequestFailedCb: func(method string, data any) {
				failedRequests.WithLabelValues(method, legacyVersion).Inc()
			},
		}
}

func makeSyncMetrics(syncReader sync.Reader, bcReader blockchain.Reader) sync.EventListener {
	opTimerHistogram := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "sync",
		Name:      "timers",
	}, []string{"op"})
	blockCount := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "sync",
		Name:      "blocks",
	})
	reorgCount := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "sync",
		Name:      "reorganisations",
	})
	chainHeightGauge := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "sync",
		Name:      "blockchain_height",
	}, func() float64 {
		height, _ := bcReader.Height()
		return float64(height)
	})
	bestBlockGauge := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "sync",
		Name:      "best_known_block_number",
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
