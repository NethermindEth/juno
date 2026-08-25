package jsonrpc

import (
	"context"
	"io"
	"maps"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils/compression"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

type HTTP struct {
	rpc    *Server
	logger log.StructuredLogger

	listener       NewRequestListener
	requestTimeout time.Duration
}

func NewHTTP(rpc *Server, logger log.StructuredLogger) *HTTP {
	return &HTTP{
		rpc:      rpc,
		logger:   logger,
		listener: &SelectiveListener{},
	}
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
			// BestSpeed: ~10x faster than the default level for ~13% larger bodies.
			gw := compression.GzipWriterLevel(writer, compression.BestSpeed)
			defer func() {
				closeErr := gw.Close()
				gw.Release()
				if closeErr != nil {
					http.Error(writer, "gzip close error", http.StatusInternalServerError)
					return
				}
			}()
			ioWriter = gw
		} else if err == nil {
			writer.Header().Set("Content-Length", strconv.Itoa(len(resp)))
		}
		_, err = ioWriter.Write(resp)
		if err != nil {
			h.logger.Warn("Failed writing response", zap.Error(err))
		}
	}
}
