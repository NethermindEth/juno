package jsonrpc

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/utils"
)

const MaxRequestBodySize = 10 * 1024 * 1024 // 10MB

var _ service.Service = (*HTTP)(nil)

type HTTP struct {
	rpc      *Server
	log      utils.SimpleLogger
	listener net.Listener
}

func NewHTTP(listener net.Listener, rpc *Server, log utils.SimpleLogger) *HTTP {
	return &HTTP{
		rpc:      rpc,
		log:      log,
		listener: listener,
	}
}

// Run starts to listen for HTTP requests
func (h *HTTP) Run(ctx context.Context) error {
	errCh := make(chan error)

	srv := &http.Server{
		Addr:    h.listener.Addr().String(),
		Handler: h,
		// ReadTimeout also sets ReadHeaderTimeout and IdleTimeout.
		ReadTimeout: 30 * time.Second,
	}

	go func() {
		<-ctx.Done()
		errCh <- srv.Shutdown(context.Background())
		close(errCh)
	}()

	if err := srv.Serve(h.listener); !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return <-errCh
}

// ServeHTTP processes an incoming HTTP request
func (h *HTTP) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
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
		_, err = writer.Write(resp)
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
		}
	}
}
