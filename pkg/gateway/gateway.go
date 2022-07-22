package gateway

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/NethermindEth/juno/internal/db/block"
	"github.com/NethermindEth/juno/internal/db/transaction"
	. "github.com/NethermindEth/juno/internal/log"
)

var (
	blockMan *block.Manager
	txMan    *transaction.Manager
)

// Server represents the gateway REST API.
type Server struct{ server *http.Server }

func New(addr string, bm *block.Manager, tm *transaction.Manager) *Server {
	blockMan = bm
	txMan = tm

	srv := &http.Server{
		Addr:    addr,
		Handler: routes(),
	}
	return &Server{srv}
}

func (g *Server) start(errCh chan<- error) {
	Logger.Info("Starting REST API.")

	// Since ListenAndServe always returns an error we need to ensure that
	// there is no write to a closed channel. Therefore, we check for
	// ErrServerClosed since that is only returned after ShutDown or
	// Closed is called. Hence, no write to the channel is required.
	// Otherwise, any other error is written which will cause the program
	// to exit.
	if err := g.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		// notest
		errCh <- errors.New("gateway: failed to start server: " + err.Error())
	}
	close(errCh)
}

func (g *Server) ListenAndServe(errCh chan<- error) {
	go g.start(errCh)
}

// Close gracefully shuts down the server.
func (g *Server) Close(timeout time.Duration) error {
	Logger.Info("Shutting down REST API.")
	ctx, _ := context.WithTimeout(context.Background(), timeout)
	return g.server.Shutdown(ctx)
}
