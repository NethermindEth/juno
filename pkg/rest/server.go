package rest

import (
	"context"
	"net/http"

	"github.com/NethermindEth/juno/internal/log"
)

// Server represents the server structure
type Server struct {
	server http.Server
}

// ListenAndServe listens to TCP port and handles requests on
// incoming connections.
func (s *Server) ListenAndServe() error {
	log.Default.Info("Listening for connections .... ")

	err := s.server.ListenAndServe()
	if err != nil {
		log.Default.With("Error", err).Error("Error occurred while trying to listen for connections.")
		return err
	}
	return nil
}

// Close shuts down the server.
func (s *Server) Close(ctx context.Context) {
	select {
	case <-ctx.Done():
		err := s.server.Shutdown(ctx)
		if err != nil {
			log.Default.With("Error", err).Info("Exiting with error.")
			return
		}
	default:
	}
}
