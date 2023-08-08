package jsonrpc

import (
	"net/http"

	"github.com/NethermindEth/juno/metrics"
	"github.com/NethermindEth/juno/utils"
	"github.com/prometheus/client_golang/prometheus"
)

const MaxRequestBodySize = 10 * 1024 * 1024 // 10MB

type HTTP struct {
	rpc      *Server
	log      utils.SimpleLogger
	requests prometheus.Counter
}

func NewHTTP(rpc *Server, log utils.SimpleLogger) *HTTP {
	h := &HTTP{
		rpc: rpc,
		log: log,
		requests: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "rpc",
			Subsystem: "http",
			Name:      "requests",
		}),
	}
	metrics.MustRegister(h.requests)
	return h
}

// ServeHTTP processes an incoming HTTP request
func (h *HTTP) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	if req.Method == "GET" {
		status := http.StatusNotFound
		if req.URL.Path == "/" {
			status = http.StatusOK
		}
		writer.WriteHeader(status)
		return
	} else if req.Method != "POST" {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	req.Body = http.MaxBytesReader(writer, req.Body, MaxRequestBodySize)
	h.requests.Inc()
	resp, err := h.rpc.HandleReader(req.Body)
	writer.Header().Set("Content-Type", "application/json")
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
	} else {
		writer.WriteHeader(http.StatusOK)
	}
	if resp != nil {
		_, err = writer.Write(resp)
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
		}
	}
}
