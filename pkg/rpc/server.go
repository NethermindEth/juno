package rpc

import (
	"context"
	"net/http"

	"github.com/NethermindEth/juno/pkg/feeder"

	"github.com/NethermindEth/juno/internal/log"
)

// Global feederClient that we use to request pending blocks
var feederClient *feeder.Client

// Server represents the server structure
type Server struct {
	server       http.Server
	feederClient *feeder.Client
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
func NewServer(addr string, client *feeder.Client) *Server {
	mux := http.NewServeMux()
	mux.Handle("/rpc", NewHandlerJsonRpc(HandlerRPC{}))
	return &Server{server: http.Server{Addr: addr, Handler: mux}, feederClient: client}
}

// ListenAndServe listens on the TCP network and handles requests on
// incoming connections.
func (s *Server) ListenAndServe() error {
	feederClient = s.feederClient
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
