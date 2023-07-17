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

type httpConn struct {
	req *http.Request
	rw  http.ResponseWriter
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
		req: req,
		rw:  rw,
	}
}

func (c *httpConn) Read(p []byte) (int, error) {
	c.req.Body = http.MaxBytesReader(c.rw, c.req.Body, MaxRequestBodySize)
	return c.req.Body.Read(p)
}

// Write returns the number of bytes of p written, not including the header.
func (c *httpConn) Write(p []byte) (int, error) {
	c.rw.Header().Set("Content-Type", "application/json")
	c.rw.WriteHeader(http.StatusOK)
	if p != nil {
		n, err := c.rw.Write(p)
		if err != nil {
			return n, err
		}
	}
	return 0, nil
}

// ServeHTTP processes an incoming HTTP request
func (h *HTTP) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	conn := accept(writer, req)
	if conn == nil {
		return
	}
	if err := h.rpc.Handle(conn); err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
	}
}
