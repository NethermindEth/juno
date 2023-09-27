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

const (
	dbLvlsAmount = 7
	dbNamespace  = "db"
)

// levelsListener listens for `db.LevelsMetrics`.
type levelsListener struct {
	numFiles        [dbLvlsAmount]prometheus.Gauge
	size            [dbLvlsAmount]prometheus.Gauge
	score           [dbLvlsAmount]prometheus.Gauge
	bytesIn         [dbLvlsAmount]prometheus.Counter
	bytesIngested   [dbLvlsAmount]prometheus.Counter
	bytesMoved      [dbLvlsAmount]prometheus.Counter
	bytesRead       [dbLvlsAmount]prometheus.Counter
	bytesCompacted  [dbLvlsAmount]prometheus.Counter
	bytesFlushed    [dbLvlsAmount]prometheus.Counter
	tablesCompacted [dbLvlsAmount]prometheus.Counter
	tablesFlushed   [dbLvlsAmount]prometheus.Counter
	tablesIngested  [dbLvlsAmount]prometheus.Counter
	tablesMoved     [dbLvlsAmount]prometheus.Counter
}

func newLevelsListener() *levelsListener {
	const subsystem = "lvl"
	listener := &levelsListener{}
	numFiles := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "num_files",
	}, []string{"lvl"})
	size := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "size",
	}, []string{"lvl"})
	score := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "score",
	}, []string{"lvl"})
	bytesIn := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "bytes_in",
	}, []string{"lvl"})
	bytesIngested := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "bytes_ingested",
	}, []string{"lvl"})
	bytesMoved := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "bytes_moved",
	}, []string{"lvl"})
	bytesRead := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "bytes_read",
	}, []string{"lvl"})
	bytesCompacted := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "bytes_compacted",
	}, []string{"lvl"})
	bytesFlushed := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "bytes_flushed",
	}, []string{"lvl"})
	tablesCompacted := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "tables_compacted",
	}, []string{"lvl"})
	tablesFlushed := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "tables_flushed",
	}, []string{"lvl"})
	tablesIngested := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "tables_ingested",
	}, []string{"lvl"})
	tablesMoved := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "tables_moved",
	}, []string{"lvl"})
	for i := 0; i < dbLvlsAmount; i++ {
		label := strconv.Itoa(i)
		listener.numFiles[i] = numFiles.WithLabelValues(label)
		listener.size[i] = size.WithLabelValues(label)
		listener.score[i] = score.WithLabelValues(label)
		listener.bytesIn[i] = bytesIn.WithLabelValues(label)
		listener.bytesIngested[i] = bytesIngested.WithLabelValues(label)
		listener.bytesMoved[i] = bytesMoved.WithLabelValues(label)
		listener.bytesRead[i] = bytesRead.WithLabelValues(label)
		listener.bytesCompacted[i] = bytesCompacted.WithLabelValues(label)
		listener.bytesFlushed[i] = bytesFlushed.WithLabelValues(label)
		listener.bytesFlushed[i] = bytesFlushed.WithLabelValues(label)
		listener.bytesFlushed[i] = bytesFlushed.WithLabelValues(label)
		listener.bytesFlushed[i] = bytesFlushed.WithLabelValues(label)
		listener.tablesCompacted[i] = tablesCompacted.WithLabelValues(label)
		listener.tablesFlushed[i] = tablesFlushed.WithLabelValues(label)
		listener.tablesIngested[i] = tablesIngested.WithLabelValues(label)
		listener.tablesMoved[i] = tablesMoved.WithLabelValues(label)
	}
	prometheus.MustRegister(
		numFiles,
		size,
		score,
		bytesIn,
		bytesIngested,
		bytesMoved,
		bytesRead,
		bytesCompacted,
		bytesFlushed,
		tablesCompacted,
		tablesFlushed,
		tablesIngested,
		tablesMoved,
	)
	return listener
}

func (listener *levelsListener) gather(metrics db.LevelsMetrics) {
	for i, lvl := range metrics.Level {
		listener.numFiles[i].Set(float64(lvl.NumFiles))
		listener.size[i].Set(float64(lvl.Size))
		listener.score[i].Set(lvl.Score)
		listener.bytesIn[i].Add(float64(lvl.BytesIn))
		listener.bytesIngested[i].Add(float64(lvl.BytesIngested))
		listener.bytesMoved[i].Add(float64(lvl.BytesMoved))
		listener.bytesRead[i].Add(float64(lvl.BytesRead))
		listener.bytesCompacted[i].Add(float64(lvl.BytesCompacted))
		listener.bytesFlushed[i].Add(float64(lvl.BytesFlushed))
		listener.tablesCompacted[i].Add(float64(lvl.TablesCompacted))
		listener.tablesFlushed[i].Add(float64(lvl.TablesFlushed))
		listener.tablesIngested[i].Add(float64(lvl.TablesIngested))
		listener.tablesMoved[i].Add(float64(lvl.TablesMoved))
	}
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
	levels := newLevelsListener()
	return &db.SelectiveListener{
		OnIOCb: func(write bool) {
			if write {
				writeCounter.Inc()
			} else {
				readCounter.Inc()
			}
		},
		OnLevelsCb: levels.gather,
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
