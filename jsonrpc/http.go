package jsonrpc

import (
	"compress/gzip"
	"context"
	"errors"
	"io"
	"maps"
	"net/http"
	"strings"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

const MaxRequestBodySize = 10 * db.Megabyte

type HTTP struct {
	rpc    *Server
	logger log.StructuredLogger

	listener       NewRequestListener
	requestTimeout time.Duration
	gate           *Gate
}

func NewHTTP(rpc *Server, logger log.StructuredLogger) *HTTP {
	h := &HTTP{
		rpc:      rpc,
		logger:   logger,
		listener: &SelectiveListener{},
	}
	return h
}

// WithListener registers a NewRequestListener
func (h *HTTP) WithListener(listener NewRequestListener) *HTTP {
	h.listener = listener
	return h
}

// WithRequestTimeout sets the maximum duration for handling an
// RPC request. Zero means no timeout.
func (h *HTTP) WithRequestTimeout(d time.Duration) *HTTP {
	h.requestTimeout = d
	return h
}

// WithGate sets an admission-control gate that bounds how many requests are
// processed concurrently (and queued) on this handler
func (h *HTTP) WithGate(g *Gate) *HTTP {
	h.gate = g
	return h
}

// ServeHTTP processes an incoming HTTP request
func (h *HTTP) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodGet {
		status := http.StatusNotFound
		if req.URL.Path == "/" {
			status = http.StatusOK
		}
		writer.WriteHeader(status)
		return
	} else if req.Method != http.MethodPost {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	ctx := req.Context()
	if h.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, h.requestTimeout)
		defer cancel()
	}

	if h.gate != nil {
		if err := h.gate.Acquire(ctx); err != nil {
			if errors.Is(err, ErrServerBusy) {
				writer.Header().Set("Retry-After", "1")
				http.Error(writer, "Too many requests", http.StatusServiceUnavailable)
				return
			}
			return
		}
		defer h.gate.Release()
	}

	req.Body = http.MaxBytesReader(writer, req.Body, MaxRequestBodySize)
	h.listener.OnNewRequest("any")

	resp, header, err := h.rpc.HandleReader(ctx, req.Body)

	writer.Header().Set("Content-Type", "application/json")
	maps.Copy(writer.Header(), header) // overwrites duplicate headers

	if err != nil {
		h.logger.Error("Handler failure", zap.Error(err))
		writer.WriteHeader(http.StatusInternalServerError)
	}
	if resp != nil {
		var ioWriter io.Writer = writer
		if strings.Contains(req.Header.Get("Accept-Encoding"), "gzip") {
			writer.Header().Set("Content-Encoding", "gzip")
			gw := gzip.NewWriter(writer)
			defer func() {
				if err := gw.Close(); err != nil {
					http.Error(writer, "gzip close error", http.StatusInternalServerError)
				}
			}()
			ioWriter = gw
		}
		_, err = ioWriter.Write(resp)
		if err != nil {
			h.logger.Warn("Failed writing response", zap.Error(err))
		}
	}
}
