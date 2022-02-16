package rpc

import (
	"context"

	cmd "github.com/NethermindEth/juno/cmd/starknet"

	pkg "github.com/NethermindEth/juno/pkg"
	"github.com/goccy/go-json"
	"github.com/osamingo/jsonrpc/v2"
)

// Handlers for getting a block by its hash / id
type BlockHashHandler struct{}
type BlockHashParams struct {
	RequestedScope pkg.RequestedScope `json:"requestedScope"`
	BlockHash      cmd.BlockHash      `json:"block_hash"`
}
type BlockHashResult pkg.BlockResponse

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

// Hanlders for getting a block by the block number
type BlockNumberHandler struct{}
type BlockNumberParams struct {
	BlockNumber    int                `json:"block_number"`
	RequestedScope pkg.RequestedScope `json:"requestedScope"`
}
type BlockNumberResult pkg.BlockResponse

func (h BlockNumberHandler) Name() string {
	return "starknet_getBlockByNumber"
}

func (h BlockNumberHandler) Params() interface{} {
	return BlockNumberParams{}
}

func (h BlockNumberHandler) Result() interface{} {
	return BlockNumberResult{}
}

func (h BlockNumberHandler) ServeJSONRPC(c context.Context, params *json.RawMessage) (interface{}, *jsonrpc.Error) {

	var p BlockNumberParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	return []string{"Response", "of", "starknet_getBlockByNumber"}, nil
}

// Hanlders for getting block transaction count by the blocks hash
type BlockTransactionCountHandler struct{}
type BlockTransactionCountParams pkg.BlockHash
type BlockTransactionCountResult cmd.BlockTransactionCount

func (h BlockTransactionCountHandler) Name() string {
	return "starknet_getBlockTransactionCountByHash"
}

func (h BlockTransactionCountHandler) Params() interface{} {
	return BlockTransactionCountParams{}
}

func (h BlockTransactionCountHandler) Result() interface{} {
	return BlockTransactionCountResult{}
}

func (h BlockTransactionCountHandler) ServeJSONRPC(c context.Context, params *json.RawMessage) (interface{}, *jsonrpc.Error) {

	var p BlockTransactionCountParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	return []string{"Response", "of", "starknet_getBlockTransactionCountByHash"}, nil
}
