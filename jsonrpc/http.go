package jsonrpc

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/NethermindEth/juno/utils"
)

const MaxRequestBodySize = 10 * 1024 * 1024 // 10MB

type Http struct {
	addr *net.TCPAddr

	rpc  *Server
	http *http.Server
	log  utils.Logger
}

func NewHttp(port uint16, methods []Method, log utils.Logger) *Http {
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

// Run starts to listen for HTTP requests
func (h *Http) Run(ctx context.Context) error {
	listener, listenErr := net.ListenTCP("tcp", h.addr)
	if listenErr != nil {
		return listenErr
	}

	go func() {
		var err error
		for ; !errors.Is(err, http.ErrServerClosed); err = h.http.Serve(listener) {
			time.Sleep(time.Second) // retry if server was not closed
		}
	}()
	go func() {
		<-ctx.Done()
		err := h.http.Shutdown(context.Background())
		if err != nil {
			h.log.Warnw("Error shutting down the http server", "err", err)
		}
	}()

	return nil
}

// ServeHTTP processes an incoming HTTP request
func (h *Http) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		req.Close = true
		return
	}

	req.Body = http.MaxBytesReader(writer, req.Body, MaxRequestBodySize)
	resp, err := h.rpc.HandleReader(req.Body)
	writer.Header().Set("Content-Type", "application/json")
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
	} else {
		writer.WriteHeader(http.StatusOK)
	}
	if resp != nil {
		writer.Write(resp)
	}
}
