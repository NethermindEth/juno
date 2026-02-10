package jsonrpc

import (
	"compress/gzip"
	"context"
	"io"
	"maps"
	"net/http"
	"strings"
	"time"

	"github.com/NethermindEth/juno/utils"
	"go.uber.org/zap"
)

const MaxRequestBodySize = 10 * utils.Megabyte

type HTTP struct {
	rpc *Server
	log utils.StructuredLogger

	listener       NewRequestListener
	requestTimeout time.Duration
}

func NewHTTP(rpc *Server, log utils.StructuredLogger) *HTTP {
	h := &HTTP{
		rpc:      rpc,
		log:      log,
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

	req.Body = http.MaxBytesReader(writer, req.Body, MaxRequestBodySize)
	h.listener.OnNewRequest("any")

	ctx := req.Context()
	if h.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, h.requestTimeout)
		defer cancel()
	}
	resp, header, err := h.rpc.HandleReader(ctx, req.Body)

	writer.Header().Set("Content-Type", "application/json")
	maps.Copy(writer.Header(), header) // overwrites duplicate headers

	if err != nil {
		h.log.Error("Handler failure", zap.Error(err))
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
			h.log.Warn("Failed writing response", zap.Error(err))
		}
	}
}
