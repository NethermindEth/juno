package client

import (
	"encoding/json"
)

//go:generate mockgen -destination=../mocks/mock_gateway_handler.go -package=mocks github.com/NethermindEth/juno/rpc GatewayInterface
type GatewayInterface interface {
	AddTransaction(txn json.RawMessage) (json.RawMessage, error)
}

type Gateway struct {
	client *Client
}

var _ GatewayInterface = &Gateway{}

func NewGateway(client *Client) *Gateway {
	return &Gateway{client: client}
}

func (g *Gateway) AddTransaction(txn json.RawMessage) (json.RawMessage, error) {
	return g.client.post(g.client.url+"/add_transaction", txn)
}
