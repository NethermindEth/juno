package testsource

import (
	"github.com/NethermindEth/juno/clients"
	"github.com/NethermindEth/juno/starknetdata/gateway"
	"github.com/NethermindEth/juno/utils"
)

type TestClientCloseFn func()

func NewTestClient(network utils.Network) (*clients.GatewayClient, TestClientCloseFn) {
	srv := newTestGatewayServer(network)
	client := clients.NewGatewayClient(srv.URL).WithBackoff(clients.NopBackoff).WithMaxRetries(0)

	return client, srv.Close
}

func NewTestGateway(network utils.Network) (*gateway.Gateway, TestClientCloseFn) {
	client, closer := NewTestClient(network)
	gw := gateway.NewGatewayWithClient(client)

	return gw, closer
}
