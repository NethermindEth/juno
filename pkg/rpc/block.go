package rpc

import (
	"context"

	cmd "github.com/NethermindEth/juno/cmd/starknet"
	"github.com/goccy/go-json"
	"github.com/osamingo/jsonrpc/v2"
)

// Handlers for getting a block by its hash / id
type BlockHashHandler struct{}

type BlockHashParams struct {
	RequestedScope cmd.RequestedScope `json:"requestedScope"`
	BlockHash      cmd.BlockHash      `json:"block_hash"`
}

type BlockHashResult cmd.BlockResponse

func (h BlockHashHandler) Name() string {
	return "starknet_getBlockByHash"
}

func (h BlockHashHandler) Params() interface{} {
	return BlockHashParams{}
}

func (h BlockHashHandler) Result() interface{} {
	return BlockHashResult{}
}

func (h BlockHashHandler) ServeJSONRPC(c context.Context, params *json.RawMessage) (interface{}, *jsonrpc.Error) {

	var p BlockHashParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	return []string{"Response", "of", "starknet_getBlockByHash"}, nil
}
