package jsonrpc

import (
	"io"
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

type httpConn struct {
	reqBody io.ReadCloser
	rw      http.ResponseWriter
}

func accept(rw http.ResponseWriter, req *http.Request) *httpConn {
	if req.Method == "GET" {
		status := http.StatusNotFound
		if req.URL.Path == "/" {
			status = http.StatusOK
		}
		rw.WriteHeader(status)
		return nil
	} else if req.Method != "POST" {
		rw.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}

	return &httpConn{
		reqBody: http.MaxBytesReader(rw, req.Body, MaxRequestBodySize),
		rw:      rw,
	}
}

func (c *httpConn) Read(p []byte) (int, error) {
	return c.reqBody.Read(p)
}

// Write returns the number of bytes of p written, not including the header.
func (c *httpConn) Write(p []byte) (int, error) {
	c.rw.Header().Set("Content-Type", "application/json")
	if p == nil {
		return 0, nil
	}
	return c.rw.Write(p)
}

// ServeHTTP processes an incoming HTTP request
func (h *HTTP) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	conn := accept(writer, req)
	if conn == nil {
		return
	}
	h.requests.Inc()
	if err := h.rpc.Handle(req.Context(), conn); err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
	}
}
