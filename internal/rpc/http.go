package rpc

import (
	"context"
	"errors"
	"net/http"
	"time"

	. "github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/jsonrpc"
	. "github.com/NethermindEth/juno/pkg/jsonrpc/providers/http"
)

type HttpRpc struct {
	server   *http.Server
	provider *HttpProvider
}

func NewHttpRpc(addr, pattern string, rpc *jsonrpc.JsonRpc) (*HttpRpc, error) {
	httpRpc := new(HttpRpc)
	httpRpc.provider = NewHttpProvider(rpc)
	mux := http.NewServeMux()
	mux.Handle(pattern, httpRpc.provider)
	httpRpc.server = &http.Server{Addr: addr, Handler: mux}
	return httpRpc, nil
}

func (h *HttpRpc) ListenAndServe(errCh chan<- error) {
	// notest
	go h.listenAndServe(errCh)
}

func (h *HttpRpc) listenAndServe(errCh chan<- error) {
	// notest
	Logger.Info("Listening for JSON-RPC connections...")

	// Since ListenAndServe always returns an error we need to ensure that there
	// is no write to a closed channel. Therefore, we check for ErrServerClosed
	// since that is only returned after ShutDown or Closed is called. Hence, no
	// write to the channel is required. Otherwise, any other error is written
	// which will cause the program to exit.
	if err := h.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		errCh <- errors.New("Failed to ListenAndServe on Metrics server: " + err.Error())
	}
	close(errCh)
}

func (h *HttpRpc) Close(timeout time.Duration) error {
	Logger.Info("Shutting down JSON-RPC server...")
	ctx, _ := context.WithTimeout(context.Background(), timeout)
	return h.server.Shutdown(ctx)
}
