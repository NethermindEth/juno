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

type HTTP struct {
	rpc    *Server
	logger log.StructuredLogger
	// For logging busy warnings without flooding
	sampledLogger log.StructuredLogger

	listener       NewRequestListener
	requestTimeout time.Duration
	gate           *Gate
}

func NewHTTP(rpc *Server, logger log.StructuredLogger) *HTTP {
	const busyLogInterval = time.Second

	h := &HTTP{
		rpc:           rpc,
		logger:        logger,
		listener:      &SelectiveListener{},
		sampledLogger: log.Sampled(logger, busyLogInterval, 1, 0),
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

func (h *HTTP) logServerBusy() {
	h.sampledLogger.Warn("Rejected RPC request: server is busy",
		zap.Int("running", h.gate.Running()),
		zap.Int("queued", h.gate.Queued()),
		zap.Uint64("rejected", h.gate.Rejected()),
	)
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

	h.listener.OnNewRequest("any")

	if h.gate != nil {
		if err := h.gate.Acquire(ctx); err != nil {
			if errors.Is(err, ErrServerBusy) {
				h.logServerBusy()
				writer.Header().Set("Retry-After", "1")
				http.Error(writer, "Server is busy", http.StatusServiceUnavailable)
				return
			}
			if errors.Is(err, context.DeadlineExceeded) {
				writer.Header().Set("Retry-After", "1")
				http.Error(writer, "Request timed out while queued", http.StatusServiceUnavailable)
				return
			}
			// context.Canceled: the client has disconnected, so there is nobody to respond to.
			return
		}
		defer h.gate.Release()
	}

	const MaxRequestBodySize = 10 * db.Megabyte
	req.Body = http.MaxBytesReader(writer, req.Body, MaxRequestBodySize)
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
