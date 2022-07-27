package gateway

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/NethermindEth/juno/internal/db/block"
	"github.com/NethermindEth/juno/internal/db/transaction"
	"github.com/NethermindEth/juno/pkg/gateway/internal/models"
	"go.uber.org/zap"
)

// gateway represents the gateway REST API.
type gateway struct {
	logger *zap.SugaredLogger
	model  models.Modeler
}

// Server holds the server that fulfils requests to the gateway API.
type Server struct {
	logger *zap.SugaredLogger
	srv    *http.Server
}

// New creates a new gateway application.
func NewServer(addr string, bm *block.Manager, tm *transaction.Manager) *Server {
	// notest
	model := &models.Model{
		BlockMan: bm,
		TxMan:    tm,
	}

	logger := NewLogger()

	gw := &gateway{
		logger: logger,
		model:  model,
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: gw.routes(),
		// TODO: The function that recovers from panics should handle this
		// capability.
		// ErrorLog: nil,
	}

	return &Server{logger: logger, srv: srv}
}

func (s *Server) start(errCh chan<- error) {
	// notest
	s.logger.Info("Starting REST API.")

	// Since ListenAndServe always returns an error we need to ensure that
	// there is no write to a closed channel. Therefore, we check for
	// ErrServerClosed since that is only returned after ShutDown or
	// Closed is called. Hence, no write to the channel is required.
	// Otherwise, any other error is written which will cause the program
	// to exit.
	if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		// notest
		errCh <- errors.New("gateway: failed to start server: " + err.Error())
	}
	close(errCh)
}

func (s *Server) ListenAndServe(errCh chan<- error) {
	// notest
	go s.start(errCh)
}

// Close gracefully shuts down the server.
func (s *Server) Close(timeout time.Duration) error {
	// notest
	s.logger.Info("Shutting down REST API.")
	ctx, _ := context.WithTimeout(context.Background(), timeout)
	return s.srv.Shutdown(ctx)
}
