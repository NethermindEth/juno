package jsonrpc

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of http requests processed",
		},
		[]string{"path"},
	)
	httpRequestFailedCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_failed_requests_total",
			Help: "Total number of failed http requests",
		},
		[]string{"path"},
	)
	rpcRequestsCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rpc_requests_total",
			Help: "Total number of rpc requests processed",
		},
		[]string{"method", "path"},
	)
	rpcRequestFailsCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rpc_failed_requests_total",
			Help: "Total number of failed rpc requests",
		},
		[]string{"method", "path"},
	)
	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_response_time_seconds",
		Help:    "Duration of HTTP requests.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})
)
