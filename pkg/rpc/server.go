package rpc

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/NethermindEth/juno/internal/db/abi"
	"github.com/NethermindEth/juno/internal/db/block"
	"github.com/NethermindEth/juno/internal/db/state"
	"github.com/NethermindEth/juno/internal/db/transaction"
	. "github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/feeder"
)

// Global feederClient that we use to request pending blocks
// todo: refactor the use of these global variables. Due to the use of
//  reflection in this package it is difficult to not use these global
//  variables. While testing it is difficult to understand where and whether
//  these variables are being set properly and thus difficult to assert the
//  validity of the testcase.
var (
	feederClient       *feeder.Client
	abiManager         *abi.Manager
	stateManager       *state.Manager
	transactionManager *transaction.Manager
	blockManager       *block.Manager
)

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
func NewServer(addr string, client *feeder.Client, abiM *abi.Manager, stateM *state.Manager,
	transactionM *transaction.Manager, blockM *block.Manager,
) *Server {
	abiManager = abiM
	stateManager = stateM
	transactionManager = transactionM
	blockManager = blockM
	mux := http.NewServeMux()
	mux.Handle("/rpc", NewHandlerJsonRpc(HandlerRPC{}))
	return &Server{server: http.Server{Addr: addr, Handler: mux}, feederClient: client}
}

// ListenAndServe listens to TCP port and handles requests on incoming
// connections. ListenAndServe runs in a separate go routine so that it doesn't
// block the caller.
func (s *Server) ListenAndServe(errCh chan<- error) {
	feederClient = s.feederClient
	// notest
	go s.listenAndServe(errCh)
}

func (s *Server) listenAndServe(errCh chan<- error) {
	Logger.Info("Listening for RPC connections...")

	// Since ListenAndServe always returns an error we need to ensure that there
	// is no write to a closed channel. Therefore, we check for ErrServerClosed
	// since that is only returned after ShutDown or Closed is called. Hence, no
	// write to the channel is required. Otherwise, any other error is written
	// which will cause the program to exit.
	if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		// notest
		errCh <- errors.New("Failed to ListenAndServe on RPC server: " + err.Error())
	}
	close(errCh)
}

// Close gracefully shuts down the server.
func (s *Server) Close(timeout time.Duration) error {
	Logger.Info("Shutting down RPC server...")
	ctx, _ := context.WithTimeout(context.Background(), timeout)
	return s.server.Shutdown(ctx)
}
