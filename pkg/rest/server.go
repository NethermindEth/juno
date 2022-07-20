package rest

import (
	"context"
	"errors"
	"net/http"
	"time"

	. "github.com/NethermindEth/juno/internal/log"
)

// Server represents the server structure
type Server struct {
	server http.Server
}

// ListenAndServe listens to TCP port and handles requests on incoming
// connections. ListenAndServe runs in a separate go routine so that it doesn't
// block the caller.
func (s *Server) ListenAndServe(errCh chan<- error) {
	// notest
	go s.listenAndServe(errCh)
}

func (s *Server) listenAndServe(errCh chan<- error) {
	Logger.Info("Listening for REST connections...")

	// Since ListenAndServe always returns an error we need to ensure that there
	// is no write to a closed channel. Therefore, we check for ErrServerClosed
	// since that is only returned after ShutDown or Closed is called. Hence, no
	// write to the channel is required. Otherwise, any other error is written
	// which will cause the program to exit.
	if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		// notest
		errCh <- errors.New("Failed to ListenAndServe on REST server: " + err.Error())
	}
	close(errCh)
}

// Close gracefully shuts down the server.
func (s *Server) Close(timeout time.Duration) error {
	Logger.Info("Shutting down REST server...")
	ctx, _ := context.WithTimeout(context.Background(), timeout)
	return s.server.Shutdown(ctx)
}
