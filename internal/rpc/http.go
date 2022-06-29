package rpc

import (
	"context"
	"net/http"

	"github.com/NethermindEth/juno/pkg/jsonrpc"
	. "github.com/NethermindEth/juno/pkg/jsonrpc/providers/http"
)

type HttpRpc struct {
	jsonRpc  *jsonrpc.Server
	server   *http.Server
	provider *HttpProvider
}

func NewHttpRpc(addr, pattern string, serviceName string, service interface{}) (*HttpRpc, error) {
	httpRpc := new(HttpRpc)
	// Create JSON-RPC 2.0 server
	httpRpc.jsonRpc = jsonrpc.NewServer()
	// Register the service
	if err := httpRpc.jsonRpc.RegisterService(serviceName, service); err != nil {
		return nil, err
	}
	// Create HTTP provider
	httpRpc.provider = NewHttpProvider(httpRpc.jsonRpc)
	// Create HTTP server
	mux := http.NewServeMux()
	mux.Handle(pattern, httpRpc.provider)
	httpRpc.server = &http.Server{Addr: addr, Handler: mux}
	return httpRpc, nil
}

func (s *HttpRpc) ListenAndServe() error {
	return s.server.ListenAndServe()
}

func (s *HttpRpc) Close(ctx context.Context) {
	s.server.Close()
}
