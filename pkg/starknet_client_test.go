package starknet_client

import (
	"context"
	"github.com/NethermindEth/juno/configs"
	"testing"
)

func newClient() (*Client, error) {
	return Dial(configs.TestRPCEndpoint)
}

func TestClient_GetBlockByHash(t *testing.T) {
	c, err := newClient()
	if err != nil {
		t.Errorf("Failed Client Creation")
	}
	defer c.Close()
	blockHash := BlockHash{
		Hash: "latest",
	}
	requestedScope := TxnHash
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
	blockHash := BlockHash{
		Hash: "latest",
	}
	requestedScope := TxnHash
	response, err := c.GetBlockByNumber(context.Background(), blockHash, requestedScope)
	if err != nil {
		t.Error(err)
	}
	t.Log(response)
}
