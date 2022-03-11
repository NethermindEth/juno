package test

import (
	"context"
	"testing"

	"github.com/NethermindEth/juno/pkg/rpc"
)

const endpoint = "https://alpha4.starknet.io"

func newClient() (*rpc.Client, error) {
	return rpc.Dial(endpoint)
}

func TestClient_GetBlockByHash(t *testing.T) {
	c, err := newClient()
	if err != nil {
		t.Errorf("Failed to initialise new client.")
	}
	defer c.Close()
	blockHash := rpc.BlockHash("latest")
	requestedScope := rpc.RequestedScope("scope")
	response, err := c.GetBlockByHash(context.Background(), blockHash, requestedScope)
	if err != nil {
		t.Error(err)
	}
	t.Log(response)
}

func TestClient_GetBlockByNumber(t *testing.T) {
	c, err := newClient()
	if err != nil {
		t.Errorf("Failed to initialise new client.")
	}
	defer c.Close()
	blockHash := rpc.BlockHash("latest")
	requestedScope := rpc.RequestedScope("scope")
	response, err := c.GetBlockByNumber(context.Background(), blockHash, requestedScope)
	if err != nil {
		t.Error(err)
	}
	t.Log(response)
}
