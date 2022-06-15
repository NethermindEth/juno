package rpc

import (
	"context"
	"net/http"

	"github.com/NethermindEth/juno/internal/log"
)

// Server represents the server structure
type Server struct {
	server http.Server
}

// HandlerRPC represents the struct that later we will apply reflection
// to call rpc methods.
type HandlerRPC struct{}

// HandlerJsonRpc contains the JSON-RPC method functions.
type HandlerJsonRpc struct {
	StructRpc interface{}
}

// NewHandlerJsonRpc creates a new HandlerJsonRpc
func NewHandlerJsonRpc(rpc interface{}) *HandlerJsonRpc {
	return &HandlerJsonRpc{StructRpc: rpc}
}

// NewServer creates a new server.
func NewServer(addr string) *Server {
	mux := http.NewServeMux()
	mux.Handle("/rpc", NewHandlerJsonRpc(HandlerRPC{}))
	return &Server{server: http.Server{Addr: addr, Handler: mux}}
}

// ListenAndServe listens on the TCP network and handles requests on
// incoming connections.
func (s *Server) ListenAndServe() error {
	// notest
	log.Default.Info("Listening for connections .... ")

	err := s.server.ListenAndServe()
	if err != nil {
		log.Default.With("Error", err).Error("Error occurred while trying to listen for connections.")
		return err
	}
	return nil
}

// Close gracefully shuts down the server.
func (s *Server) Close(ctx context.Context) {
	// notest
	log.Default.Info("Closing RPC server")
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
