package jsonrpc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/NethermindEth/juno/utils"
)

const MaxRequestBodySize = 10 * 1024 * 1024 // 10MB

type HTTP struct {
	rpc  *Server
	http *http.Server
	log  utils.SimpleLogger
}

func NewHTTP(port uint16, methods []Method, log utils.SimpleLogger) *HTTP {
	headerTimeout := 1 * time.Second
	h := &HTTP{
		rpc: NewServer(),
		log: log,
	}
	h.http = &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           h,
		ReadHeaderTimeout: headerTimeout,
	}
	for _, method := range methods {
		err := h.rpc.RegisterMethod(method)
		if err != nil {
			panic(err)
		}
	}
	return h
}

// Run starts to listen for HTTP requests
func (h *HTTP) Run(ctx context.Context) error {
	errCh := make(chan error)

	go func() {
		<-ctx.Done()
		errCh <- h.http.Shutdown(context.Background())
		close(errCh)
	}()

	if err := h.http.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
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
