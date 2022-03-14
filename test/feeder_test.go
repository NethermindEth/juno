package test

import (
	"github.com/NethermindEth/juno/pkg/gateway"
	"testing"
)

func setupClient() *gateway.Client {
	return gateway.NewClient("http://alpha4.starknet.io")
}

func TestGatewayClient(t *testing.T) {
	_ = setupClient()
}
