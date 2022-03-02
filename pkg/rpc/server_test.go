package rpc

import (
	"context"
	cmd "github.com/NethermindEth/juno/cmd/starknet"
	"testing"
)

var handlerRpc *HandlerRPC

func init() {
	handlerRpc = getHandler()
}

func getHandler() *HandlerRPC {
	return &HandlerRPC{}
}

type starknetCallParams struct {
	ctx       context.Context
	request   cmd.FunctionCall
	blockHash cmd.BlockHashOrTag
}

type starknetCallResponse struct {
	result cmd.ResultCall
	error  error
}

func checkResultCallEqual(r1, r2 cmd.ResultCall) bool {
	if len(r1) != len(r2) {
		return false
	}
	for i, v := range r1 {
		if v != r2[i] {
			return false
		}
	}
	return true
}

func TestStarknetCall(t *testing.T) {
	test := []struct {
		params   starknetCallParams
		response starknetCallResponse
	}{
		{
			starknetCallParams{
				ctx:       context.Background(),
				request:   cmd.FunctionCall{},
				blockHash: cmd.BlockHashOrTag{},
			},
			starknetCallResponse{
				result: []string{"Response", "of", "starknet_call"},
				error:  nil,
			},
		},
	}
	for _, v := range test {
		r, err := handlerRpc.StarknetCall(v.params.ctx, v.params.request, v.params.blockHash)
		if !checkResultCallEqual(r, v.response.result) || err != v.response.error {
			t.Fail()
		}
	}
}
