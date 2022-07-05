package rest

import (
	"context"
	"net/http"
	"time"

	"github.com/NethermindEth/juno/internal/log"
)

// Server represents the server structure
type Server struct {
	server http.Server
}

// ListenAndServe listens to TCP port and handles requests on
// incoming connections.
func (s *Server) ListenAndServe() error {
	// notest
	log.Default.Info("Rest server listening for connections .... ")

	return s.server.ListenAndServe()
}

// Close shuts down the server.
func (s *Server) Close(timeout time.Duration) error {
	ctx := context.Background()
	ctx, _ = context.WithTimeout(ctx, timeout)
	return s.server.Shutdown(ctx)
}
