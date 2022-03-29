package rpc

import (
	"context"
	"testing"
)

const endpoint = "https://alpha4.starknet.io"

func newClient() (*Client, error) {
	return Dial(endpoint)
}

func TestClient_GetBlockByHash(t *testing.T) {
	c, err := newClient()
	if err != nil {
		t.Error("failed to initialise new client.")
	}
	defer c.Close()
	blockHash := BlockHash("latest")
	requestedScope := RequestedScope("scope")
	response, err := c.GetBlockByHash(
		context.Background(), blockHash, requestedScope)
	if err != nil {
		t.Error(err)
	}
	t.Log(response)
}

func TestClient_GetBlockByNumber(t *testing.T) {
	c, err := newClient()
	if err != nil {
		t.Error("failed to initialise new client.")
	}
	defer c.Close()
	blockHash := BlockHash("latest")
	requestedScope := RequestedScope("scope")
	response, err := c.GetBlockByNumber(
		context.Background(), blockHash, requestedScope)
	if err != nil {
		t.Error(err)
	}
	t.Log(response)
}
