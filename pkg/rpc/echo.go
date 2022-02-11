package rpc

import (
	"context"
	"github.com/goccy/go-json"
	"github.com/osamingo/jsonrpc/v2"
)

type EchoHandler struct{}
type (
	EchoParams struct {
		Name string `json:"name"`
	}
	EchoResult struct {
		Message string `json:"message"`
	}
)

func (h EchoHandler) Name() string {
	return "echo"
}

func (h EchoHandler) Params() interface{} {
	return EchoParams{}
}

func (h EchoHandler) Result() interface{} {
	return EchoResult{}
}

func (h EchoHandler) ServeJSONRPC(c context.Context, params *json.RawMessage) (interface{}, *jsonrpc.Error) {

	var p EchoParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	return EchoResult{
		Message: "Hello, " + p.Name,
	}, nil
}
