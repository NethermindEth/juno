package testsource

import (
	"io"
	"net/http/httptest"

	"github.com/NethermindEth/juno/clients"
	"github.com/NethermindEth/juno/starknetdata/gateway"
	"github.com/NethermindEth/juno/utils"
)

type srvCloser struct {
	srv *httptest.Server
}

func (c *srvCloser) Close() error {
	c.srv.Close()
	return nil
}

func NewTestClient(network utils.Network) (*clients.GatewayClient, io.Closer) {
	srv := newTestGatewayServer(network)
	client := clients.NewGatewayClient(srv.URL)

	return client, &srvCloser{srv}
}

func NewTestGateway(network utils.Network) (*gateway.Gateway, io.Closer) {
	client, closer := NewTestClient(network)
	gw := gateway.NewGatewayWithClient(client)

	return gw, closer
}
