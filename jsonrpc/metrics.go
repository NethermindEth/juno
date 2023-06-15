package jsonrpc

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	rpcRequestsCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rpc_requests_total",
			Help: "Total number of rpc requests processed",
		},
		[]string{"method", "path"},
	)
	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_response_time_seconds",
		Help:    "Duration of HTTP requests.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})
)
