package rpc

import (
	"context"
	cmd "github.com/NethermindEth/juno/cmd/starknet"
	"github.com/goccy/go-json"
	"github.com/osamingo/jsonrpc/v2"
)

type CallHandler struct{}
type CallParams cmd.RequestRPC
type CallResult cmd.ResultCall

func (h CallHandler) Name() string {
	return "starknet_call"
}

func (h CallHandler) Params() interface{} {
	return CallParams{}
}

func (h CallHandler) Result() interface{} {
	return CallResult{}
}

func (h CallHandler) ServeJSONRPC(c context.Context, params *json.RawMessage) (interface{}, *jsonrpc.Error) {

	var p CallParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	return []string{"Response", "of", "starknet_call"}, nil
}
