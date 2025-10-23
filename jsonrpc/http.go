package jsonrpc

import (
	"compress/gzip"
	"io"
	"maps"
	"net/http"
	"strings"

	"github.com/NethermindEth/juno/utils"
)

const MaxRequestBodySize = 10 * utils.Megabyte

type HTTP struct {
	rpc *Server
	log utils.SimpleLogger

	listener NewRequestListener
}

func NewHTTP(rpc *Server, log utils.SimpleLogger) *HTTP {
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

	if req.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(req.Body)
		if err != nil {
			http.Error(writer, "invalid gzip", http.StatusBadRequest)
			return
		}
		req.Body = gz
	}
	req.Body = http.MaxBytesReader(writer, req.Body, MaxRequestBodySize)
	h.listener.OnNewRequest("any")
	resp, header, err := h.rpc.HandleReader(req.Context(), req.Body)

	writer.Header().Set("Content-Type", "application/json")
	maps.Copy(writer.Header(), header) // overwrites duplicate headers

	if err != nil {
		h.log.Errorw("Handler failure", "err", err)
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
			h.log.Warnw("Failed writing response", "err", err)
		}
	}
}
