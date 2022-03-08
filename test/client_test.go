package test

import (
	"context"
	"github.com/NethermindEth/juno/pkg/rpc"
	"github.com/NethermindEth/juno/pkg/types"
	"testing"
)

const TestRPCEndpoint = "https://alpha4.starknet.io"

func newClient() (*rpc.Client, error) {
	return rpc.Dial(TestRPCEndpoint)
}

func TestClient_GetBlockByHash(t *testing.T) {
	c, err := newClient()
	if err != nil {
		t.Errorf("Failed Client Creation")
	}
	defer c.Close()
	blockHash := types.BlockHash("latest")
	requestedScope := types.RequestedScope("scope")
	response, err := c.GetBlockByHash(context.Background(), blockHash, requestedScope)
	if err != nil {
		t.Error(err)
	}
	t.Log(response)
}

func TestClient_GetBlockByNumber(t *testing.T) {
	c, err := newClient()
	if err != nil {
		t.Errorf("Failed Client Creation")
	}
	defer c.Close()
	blockHash := types.BlockHash("latest")
	requestedScope := types.RequestedScope("scope")
	response, err := c.GetBlockByNumber(context.Background(), blockHash, requestedScope)
	if err != nil {
		t.Error(err)
	}
	t.Log(response)
}
