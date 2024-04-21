package jsonrpc

import (
	"net/http"

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

	req.Body = http.MaxBytesReader(writer, req.Body, MaxRequestBodySize)
	h.listener.OnNewRequest("any")
	resp, err := h.rpc.HandleReader(req.Context(), req.Body)
	writer.Header().Set("Content-Type", "application/json")
	if err != nil {
		h.log.Errorw("Handler failure", "err", err)
		writer.WriteHeader(http.StatusInternalServerError)
	}
	if resp != nil {
		_, err = writer.Write(resp)
		if err != nil {
			h.log.Warnw("Failed writing response", "err", err)
		}
	}
}
