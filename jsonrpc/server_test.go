package jsonrpc_test

import (
	"testing"

	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_RegisterMethod(t *testing.T) {
	server := jsonrpc.NewServer()
	tests := map[string]struct {
		handler    any
		paramNames []jsonrpc.Parameter
		want       string
	}{
		"not a func handler": {
			handler: 44,
			want:    "handler must be a function",
		},
		"excess param names": {
			handler:    func() {},
			paramNames: []jsonrpc.Parameter{{Name: "param1"}},
			want:       "number of function params and param names must match",
		},
		"missing param names": {
			handler:    func(param1, param2 int) {},
			paramNames: []jsonrpc.Parameter{{Name: "param1"}},
			want:       "number of function params and param names must match",
		},
		"no return": {
			handler: func(param1, param2 int) {},
			want:    "handler must return 2 values",
		},
		"int return": {
			handler: func(param1, param2 int) (int, int) { return 0, 0 },
			want:    "second return value must be a *jsonrpc.Error",
		},
		"no error return": {
			handler: func(param1, param2 int) (any, int) { return 0, 0 },
			want:    "second return value must be a *jsonrpc.Error",
		},
	}

	for desc, test := range tests {
		err := server.RegisterMethod("method", test.paramNames, test.handler)
		assert.EqualError(t, err, test.want, desc)
	}

	err := server.RegisterMethod("method", nil, func(param1, param2 int) (int, *jsonrpc.Error) { return 0, nil })
	assert.NoError(t, err)
}

func TestHandle(t *testing.T) {
	server := jsonrpc.NewServer()
	require.NoError(t, server.RegisterMethod("method", []jsonrpc.Parameter{{Name: "num"}, {Name: "shouldError", Optional: true}, {Name: "msg", Optional: true}},
		func(num *int, shouldError bool, msg string) (any, *jsonrpc.Error) {
			if shouldError {
				return nil, &jsonrpc.Error{Code: 44, Message: msg}
			}
			return struct {
				Doubled int `json:"doubled"`
			}{*num * 2}, nil
		}))

	tests := []struct {
		req string
		res string
	}{
		{
			req: `{]`,
			res: `{"jsonrpc":"2.0","error":{"code":-32600,"message":"invalid character ']' looking for beginning of object key string"},"id":null}`,
		},
		{
			req: `{"jsonrpc" : "1.0", "id" : 1}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32600,"message":"unsupported RPC request version"},"id":null}`,
		},
		{
			req: `{"jsonrpc" : "2.0", "method" : "doesnotexits" , "id" : 2}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32601,"message":"method not found"},"id":2}`,
		},
		{
			req: `{"jsonrpc" : "2.0", "method" : "method", "params" : [3, false] , "id" : 3}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32602,"message":"missing param in list"},"id":3}`,
		},
		{
			req: `{"jsonrpc" : "2.0", "method" : "method", "params" : [3, false, "error message"] , "id" : 3}`,
			res: `{"jsonrpc":"2.0","result":{"doubled":6},"id":3}`,
		},
		{
			req: `{"jsonrpc" : "2.0", "method" : "method", "params" : [3, true, "error message"] , "id" : 4}`,
			res: `{"jsonrpc":"2.0","error":{"code":44,"message":"error message"},"id":4}`,
		},
		{
			req: `{"jsonrpc" : "2.0", "method" : "method",
					"params" : { "num" : 5, "shouldError" : false, "msg": "error message" } , "id" : 5}`,
			res: `{"jsonrpc":"2.0","result":{"doubled":10},"id":5}`,
		},
		{
			req: `{"jsonrpc" : "2.0", "method" : "method",
					"params" : { "num" : 5 } , "id" : 5}`,
			res: `{"jsonrpc":"2.0","result":{"doubled":10},"id":5}`,
		},
		{
			req: `{"jsonrpc" : "2.0", "method" : "method",
					"params" : { "num" : 5, "shouldError" : true } , "id" : 22}`,
			res: `{"jsonrpc":"2.0","error":{"code":44,"message":""},"id":22}`,
		},
		{
			req: `{"jsonrpc" : "2.0", "method" : "method",
					"params" : { "shouldError" : true } , "id" : 22}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32602,"message":"missing non-optional param"},"id":22}`,
		},
		{
			req: `[]`,
			res: `[]`,
		},
		{
			req: `[{"jsonrpc" : "2.0", "method" : "method",
					"params" : { "num" : 5 } , "id" : 5}]`,
			res: `[{"jsonrpc":"2.0","result":{"doubled":10},"id":5}]`,
		},
		{
			req: `[{"jsonrpc" : "2.0", "method" : "method",
					"params" : { "num" : 5 } , "id" : 5},
					{"jsonrpc" : "2.0", "method" : "method",
					"params" : { "num" : 44 } , "id" : 6}]`,
			res: `[{"jsonrpc":"2.0","result":{"doubled":10},"id":5},{"jsonrpc":"2.0","result":{"doubled":88},"id":6}]`,
		},
		{
			req: `[{"jsonrpc" : "2.0", "method" : "method",
					"params" : { "num" : 5 } , "id" : 5},
					{"jsonrpc" : "2.0", "method" : "fail",
					"params" : { "num" : 5 } , "id" : 7},
					{"jsonrpc" : "2.0", "method" : "method",
					"params" : { "num" : 44 } , "id" : 6}]`,
			res: `[{"jsonrpc":"2.0","result":{"doubled":10},"id":5},{"jsonrpc":"2.0","error":{"code":-32601,"message":"method not found"},"id":7},{"jsonrpc":"2.0","result":{"doubled":88},"id":6}]`,
		},
	}

	for _, test := range tests {
		res, err := server.Handle([]byte(test.req))
		assert.NoError(t, err)
		assert.Equal(t, test.res, string(res))
	}
}
