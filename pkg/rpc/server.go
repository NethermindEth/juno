package rpc

import (
	"context"
	"net/http"
	"time"

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
	log.Default.Info("RPC server listening for connections .... ")

	return s.server.ListenAndServe()
}

// Close gracefully shuts down the server, otherwise exit after 5 seconds.
func (s *Server) Close(timeout time.Duration) error {
	ctx := context.Background()
	ctx, _ = context.WithTimeout(ctx, timeout)
	return s.server.Shutdown(ctx)
}
