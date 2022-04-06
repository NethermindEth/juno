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
		t.Error("Failed to initialise a new client.")
	}
	defer c.Close()
	blockHash := BlockHash("latest")
	requestedScope := RequestedScope("scope")
	response, err := c.GetBlockByHash(
		context.Background(), blockHash, requestedScope)
	if err != nil {
		t.Error("Failed to get block by hash.")
	}
	t.Log(response)
}

func TestClient_GetBlockByNumber(t *testing.T) {
	c, err := newClient()
	if err != nil {
		t.Error("Failed to initialise new a client.")
	}
	defer c.Close()
	blockHash := BlockHash("latest")
	requestedScope := RequestedScope("scope")
	response, err := c.GetBlockByNumber(
		context.Background(), blockHash, requestedScope)
	if err != nil {
		t.Error("Failed to get block by number.")
	}
	t.Log(response)
}
