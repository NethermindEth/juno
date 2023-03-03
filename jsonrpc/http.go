package jsonrpc

import (
	"context"
	"net"
	"net/http"

	"github.com/NethermindEth/juno/utils"
)

type Http struct {
	addr *net.TCPAddr

	rpc  *Server
	http *http.Server
	log  utils.Logger
}

func New(port uint16, methods []Method, log utils.Logger) *Http {
	h := &Http{
		rpc: NewServer(),
		addr: &net.TCPAddr{
			IP:   net.IPv4zero,
			Port: int(port),
		},
		http: &http.Server{},
		log:  log,
	}
	h.http.Handler = h
	for _, method := range methods {
		err := h.rpc.RegisterMethod(method)
		if err != nil {
			panic(err)
		}
	}
	return h
}

// Serve starts to listen for HTTP requests
func (h *Http) Serve() error {
	listener, listenErr := net.ListenTCP("tcp", h.addr)
	if listenErr != nil {
		return listenErr
	}
	h.log.Infow("Starting RPC server", "addr", h.addr)
	return h.http.Serve(listener)
}

func (h *Http) Shutdown() error {
	return h.http.Shutdown(context.Background())
}

// ServeHTTP processes an incoming HTTP request
func (h *Http) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		req.Close = true
		return
	}

	resp, err := h.rpc.HandleReader(req.Body)
	writer.Header().Set("Content-Type", "application/json")
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		h.log.Warnw("Error: rpc.HandlerReader", "err", err)
	} else {
		writer.WriteHeader(http.StatusOK)
	}
	if resp != nil {
		if _, err = writer.Write(resp); err != nil {
			h.log.Warnw("Error: http.ServeHTTP", "err", err)
		}
	}
}
