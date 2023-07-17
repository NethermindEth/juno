package jsonrpc_test

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"testing"

	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/utils"
	"github.com/go-playground/validator/v10"
	"github.com/sourcegraph/conc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_RegisterMethod(t *testing.T) {
	server := jsonrpc.NewServer(1, utils.NewNopZapLogger())
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
			want:       "number of non-context function params and param names must match",
		},
		"missing param names": {
			handler:    func(param1, param2 int) {},
			paramNames: []jsonrpc.Parameter{{Name: "param1"}},
			want:       "number of non-context function params and param names must match",
		},
		"no return": {
			handler:    func(param1, param2 int) {},
			paramNames: []jsonrpc.Parameter{{Name: "param1"}, {Name: "param2"}},
			want:       "handler must return 2 values",
		},
		"int return": {
			handler:    func(param1, param2 int) (int, int) { return 0, 0 },
			paramNames: []jsonrpc.Parameter{{Name: "param1"}, {Name: "param2"}},
			want:       "second return value must be a *jsonrpc.Error",
		},
		"no error return": {
			handler:    func(param1, param2 int) (any, int) { return 0, 0 },
			paramNames: []jsonrpc.Parameter{{Name: "param1"}, {Name: "param2"}},
			want:       "second return value must be a *jsonrpc.Error",
		},
	}

	for desc, test := range tests {
		t.Run(desc, func(t *testing.T) {
			err := server.RegisterMethod(jsonrpc.Method{
				Name:    "method",
				Params:  test.paramNames,
				Handler: test.handler,
			})
			assert.EqualError(t, err, test.want, desc)
		})
	}

	t.Run("should not fail", func(t *testing.T) {
		err := server.RegisterMethod(jsonrpc.Method{
			Name:    "method",
			Params:  []jsonrpc.Parameter{{Name: "param1"}, {Name: "param2"}},
			Handler: func(param1, param2 int) (int, *jsonrpc.Error) { return 0, nil },
		})
		assert.NoError(t, err)
	})
}

func TestHandle(t *testing.T) {
	type validationStruct struct {
		A int `validate:"min=1"`
	}
	methods := []jsonrpc.Method{
		{
			Name:   "method",
			Params: []jsonrpc.Parameter{{Name: "num"}, {Name: "shouldError", Optional: true}, {Name: "msg", Optional: true}},
			Handler: func(num *int, shouldError bool, data any) (any, *jsonrpc.Error) {
				if shouldError {
					return nil, &jsonrpc.Error{Code: 44, Message: "Expected Error", Data: data}
				}
				return struct {
					Doubled int `json:"doubled"`
				}{*num * 2}, nil
			},
		},
		{
			Name:   "subtract",
			Params: []jsonrpc.Parameter{{Name: "minuend"}, {Name: "subtrahend"}},
			Handler: func(a, b int) (int, *jsonrpc.Error) {
				return a - b, nil
			},
		},
		{
			Name:   "update",
			Params: []jsonrpc.Parameter{{Name: "a"}, {Name: "b"}, {Name: "c"}, {Name: "d"}, {Name: "e"}},
			Handler: func(a, b, c, d, e int) (int, *jsonrpc.Error) {
				return 0, nil
			},
		},
		{
			Name:   "foobar",
			Params: []jsonrpc.Parameter{},
			Handler: func() (int, *jsonrpc.Error) {
				return 0, nil
			},
		},
		{
			Name:   "validation",
			Params: []jsonrpc.Parameter{{Name: "param"}},
			Handler: func(v validationStruct) (int, *jsonrpc.Error) {
				return v.A, nil
			},
		},
		{
			Name:   "validationSlice",
			Params: []jsonrpc.Parameter{{Name: "param"}},
			Handler: func(v []validationStruct) (int, *jsonrpc.Error) {
				return v[0].A, nil
			},
		},
		{
			Name:   "validationPointer",
			Params: []jsonrpc.Parameter{{Name: "param"}},
			Handler: func(v *validationStruct) (int, *jsonrpc.Error) {
				return v.A, nil
			},
		},
		{
			Name:   "validationMapPointer",
			Params: []jsonrpc.Parameter{{Name: "param"}},
			Handler: func(v map[string]*validationStruct) (int, *jsonrpc.Error) {
				return v["expectedkey"].A, nil
			},
		},
		{
			Name:    "acceptsContext",
			Handler: func(_ context.Context) (int, *jsonrpc.Error) { return 0, nil },
		},
		{
			Name:   "acceptsContextAndTwoParams",
			Params: []jsonrpc.Parameter{{Name: "a"}, {Name: "b"}},
			Handler: func(_ context.Context, a, b int) (int, *jsonrpc.Error) {
				return b - a, nil
			},
		},
	}
	server := jsonrpc.NewServer(1, utils.NewNopZapLogger()).WithValidator(validator.New())
	for _, m := range methods {
		require.NoError(t, server.RegisterMethod(m))
	}

	tests := map[string]struct {
		isBatch bool
		req     string
		res     string
	}{
		"invalid json": {
			req: `{]`,
			res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"invalid character ']' looking for beginning of object key string"},"id":null}`,
		},
		"invalid json batch path": {
			req: `[{]`,
			res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"invalid character ']' looking for beginning of object key string"},"id":null}`,
		},
		"wrong version": {
			req: `{"jsonrpc" : "1.0", "id" : 1}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request","data":"unsupported RPC request version"},"id":1}`,
		},
		"wrong version with null id": {
			req: `{"jsonrpc" : "1.0", "id" : null}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request","data":"unsupported RPC request version"},"id":null}`,
		},
		"non existent method": {
			req: `{"jsonrpc" : "2.0", "method" : "doesnotexits" , "id" : 2}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32601,"message":"Method Not Found"},"id":2}`,
		},
		"no params": {
			req: `{"jsonrpc" : "2.0", "method" : "method", "id" : 5}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params","data":"missing non-optional param field"},"id":5}`,
		},
		"missing param(s)": {
			req: `{"jsonrpc" : "2.0", "method" : "method", "params" : [3, false] , "id" : 3}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params","data":"missing/unexpected params in list"},"id":3}`,
		},
		"too many params": {
			req: `{"jsonrpc" : "2.0", "method" : "method", "params" : [3, false, "error message", "too many"] , "id" : 3}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params","data":"missing/unexpected params in list"},"id":3}`,
		},
		"list params": {
			req: `{"jsonrpc" : "2.0", "method" : "method", "params" : [3, false, "error message"] , "id" : 3}`,
			res: `{"jsonrpc":"2.0","result":{"doubled":6},"id":3}`,
		},
		"list params, should soft error": {
			req: `{"jsonrpc" : "2.0", "method" : "method", "params" : [3, true, "error message"] , "id" : 4}`,
			res: `{"jsonrpc":"2.0","error":{"code":44,"message":"Expected Error","data":"error message"},"id":4}`,
		},
		"named params": {
			req: `{"jsonrpc" : "2.0", "method" : "method",
					"params" : { "num" : 5, "shouldError" : false, "msg": "error message" } , "id" : 5}`,
			res: `{"jsonrpc":"2.0","result":{"doubled":10},"id":5}`,
		},
		"named params with defaults": {
			req: `{"jsonrpc" : "2.0", "method" : "method",
					"params" : { "num" : 5 } , "id" : 5}`,
			res: `{"jsonrpc":"2.0","result":{"doubled":10},"id":5}`,
		},
		"named params, should soft error": {
			req: `{"jsonrpc" : "2.0", "method" : "method",
					"params" : { "num" : 5, "shouldError" : true } , "id" : 22}`,
			res: `{"jsonrpc":"2.0","error":{"code":44,"message":"Expected Error"},"id":22}`,
		},
		"missing nonoptional param": {
			req: " \r\t\n" + `{"jsonrpc" : "2.0", "method" : "method",
					"params" : { "shouldError" : true } , "id" : 22}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params","data":"missing non-optional param"},"id":22}`,
		},
		"empty batch": {
			req: `[]`,
			res: `{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request","data":"empty batch"},"id":null}`,
		},
		"single request in batch": {
			req: " \r\t\n" + `[{"jsonrpc" : "2.0", "method" : "method",
					"params" : { "num" : 5 } , "id" : 5}]`,
			res: `[{"jsonrpc":"2.0","result":{"doubled":10},"id":5}]`,
		},
		"multiple requests in batch": {
			isBatch: true,
			req: `[{"jsonrpc" : "2.0", "method" : "method",
					"params" : { "num" : 5 } , "id" : 5},
					{"jsonrpc" : "2.0", "method" : "method",
					"params" : { "num" : 44 } , "id" : 6}]`,
			res: `[{"jsonrpc":"2.0","result":{"doubled":10},"id":5},{"jsonrpc":"2.0","result":{"doubled":88},"id":6}]`,
		},
		"failing and successful requests mixed in a batch": {
			isBatch: true,
			req: `[{"jsonrpc" : "2.0", "method" : "method",
					"params" : { "num" : 5 } , "id" : 5},
					{"jsonrpc" : "2.0", "method" : "fail",
					"params" : { "num" : 5 } , "id" : 7},
					{"jsonrpc" : "2.0", "method" : "method",
					"params" : { "num" : 44 } , "id" : 6}]`,
			res: `[{"jsonrpc":"2.0","result":{"doubled":10},"id":5},{"jsonrpc":"2.0","error":{"code":-32601,"message":"Method Not Found"},"id":7},{"jsonrpc":"2.0","result":{"doubled":88},"id":6}]`,
		},
		"notification": {
			req: `{"jsonrpc" : "2.0", "method" : "method","params" : { "num" : 5, "shouldError" : false, "msg": "error message" }}`,
			res: ``,
		},
		"batch with notif and string id": {
			isBatch: true,
			req: `[{"jsonrpc" : "2.0", "method" : "method",
					"params" : { "num" : 5 }},
					{"jsonrpc" : "2.0", "method" : "fail",
					"params" : { "num" : 5 } , "id" : "7"},
					{"jsonrpc" : "2.0", "method" : "method",
					"params" : { "num" : 44 } , "id" : 6}]`,
			res: `[{"jsonrpc":"2.0","error":{"code":-32601,"message":"Method Not Found"},"id":"7"},{"jsonrpc":"2.0","result":{"doubled":88},"id":6}]`,
		},
		"batch with all notifs": {
			req: `[{"jsonrpc" : "2.0", "method" : "method",
					"params" : { "num" : 5 }},
					{"jsonrpc" : "2.0", "method" : "method",
					"params" : { "num" : 44 }}]`,
			res: ``,
		},
		"nested batch": {
			req: `[[{"jsonrpc" : "2.0", "method" : "method",
					"params" : { "num" : 5 }}],
					[{"jsonrpc" : "2.0", "method" : "method",
					"params" : { "num" : 44 }}]]`,
			res: `[{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request","data":"json: cannot unmarshal array into Go value of type jsonrpc.request"},"id":null},{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request","data":"json: cannot unmarshal array into Go value of type jsonrpc.request"},"id":null}]`,
		},
		"no method": {
			req: `{
					"jsonrpc" : "2.0"
				}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request","data":"no method specified"},"id":null}`,
		},
		"number param": {
			req: `
				{
					"jsonrpc" : "2.0",
					"method" : "rpc_call",
					"params" : 44
				}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request","data":"params should be an array or an object"},"id":null}`,
		},
		"string param": {
			req: `
				{
					"jsonrpc" : "2.0",
					"method" : "rpc_call",
					"params" : "44"
				}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request","data":"params should be an array or an object"},"id":null}`,
		},
		"array id": {
			req: `
				{
					"jsonrpc" : "2.0",
					"method" : "rpc_call",
					"params" : { "malatya" : "44"},
					"id"     : [37]
				}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request","data":"id should be a string or an integer"},"id":null}`,
		},
		"map id": {
			req: `
				{
					"jsonrpc" : "2.0",
					"method" : "rpc_call",
					"params" : { "malatya" : "44"},
					"id"     : { "44" : "37"}
				}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request","data":"id should be a string or an integer"},"id":null}`,
		},
		"float id": {
			req: `
				{
					"jsonrpc" : "2.0",
					"method" : "rpc_call",
					"params" : { "malatya" : "44"},
					"id"     : 44.37
				}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request","data":"id should be a string or an integer"},"id":null}`,
		},
		"wrong param type": {
			req: `{"jsonrpc" : "2.0", "method" : "method", "params" : ["3", false, "error message"] , "id" : 3}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params","data":"json: cannot unmarshal string into Go value of type int"},"id":3}`,
		},
		"multiple versions in batch": {
			isBatch: true,
			req: `[{"jsonrpc" : "1.0", "method" : "method",
					"params" : { "num" : 5 } , "id" : 5},
					{"jsonrpc" : "2.0", "method" : "method",
					"params" : { "num" : 44 } , "id" : 6}]`,
			res: `[{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request","data":"unsupported RPC request version"},"id":5},{"jsonrpc":"2.0","result":{"doubled":88},"id":6}]`,
		},
		"invalid value in struct": {
			req: `{"jsonrpc" : "2.0", "method" : "validation", "params" : [ {"A": 0} ], "id" : 1}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params","data":"Key: 'validationStruct.A' Error:Field validation for 'A' failed on the 'min' tag"},"id":1}`,
		},
		"valid value in struct": {
			req: `{"jsonrpc" : "2.0", "method" : "validation", "params" : [{"A": 1}], "id" : 1}`,
			res: `{"jsonrpc":"2.0","result":1,"id":1}`,
		},
		"invalid value in struct pointer": {
			req: `{"jsonrpc" : "2.0", "method" : "validationPointer", "params" : [ {"A": 0} ], "id" : 1}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params","data":"Key: 'validationStruct.A' Error:Field validation for 'A' failed on the 'min' tag"},"id":1}`,
		},
		"valid value in struct pointer": {
			req: `{"jsonrpc" : "2.0", "method" : "validationPointer", "params" : [ {"A": 1} ], "id" : 1}`,
			res: `{"jsonrpc":"2.0","result":1,"id":1}`,
		},
		"invalid value in slice struct": {
			req: `{"jsonrpc" : "2.0", "method" : "validationSlice", "params" : [ [{"A": 0}] ], "id" : 1}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params","data":"Key: 'validationStruct.A' Error:Field validation for 'A' failed on the 'min' tag"},"id":1}`,
		},
		"valid value in slice of struct": {
			req: `{"jsonrpc" : "2.0", "method" : "validationSlice", "params" : [[{"A": 1}]], "id" : 1}`,
			res: `{"jsonrpc":"2.0","result":1,"id":1}`,
		},
		"invalid value in map of pointer": {
			req: `{"jsonrpc" : "2.0", "method" : "validationMapPointer", "params" : [ { "notthexpectedkey" : {"A": 0}} ], "id" : 1}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params","data":"Key: 'validationStruct.A' Error:Field validation for 'A' failed on the 'min' tag"},"id":1}`,
		},
		"valid value in map of pointer": {
			req: `{"jsonrpc" : "2.0", "method" : "validationMapPointer", "params" : [ { "expectedkey" : {"A": 1}} ], "id" : 1}`,
			res: `{"jsonrpc":"2.0","result":1,"id":1}`,
		},
		"handler accepts context with array params": {
			req: `{"jsonrpc": "2.0", "method": "acceptsContext", "params": [], "id": 1}`,
			res: `{"jsonrpc":"2.0","result":0,"id":1}`,
		},
		"handler accepts context and two params with array params": {
			req: `{"jsonrpc": "2.0", "method": "acceptsContextAndTwoParams", "params": [1, 3], "id": 1}`,
			res: `{"jsonrpc":"2.0","result":2,"id":1}`,
		},
		"handler accepts context with named params": {
			req: `{"jsonrpc": "2.0", "method": "acceptsContext", "params": {}, "id": 1}`,
			res: `{"jsonrpc":"2.0","result":0,"id":1}`,
		},
		"handler accepts context and two params with named params": {
			req: `{"jsonrpc": "2.0", "method": "acceptsContextAndTwoParams", "params": {"b": 3, "a": 1}, "id": 1}`,
			res: `{"jsonrpc":"2.0","result":2,"id":1}`,
		},
		// spec tests
		"rpc call with positional parameters 1": {
			req: `{"jsonrpc": "2.0", "method": "subtract", "params": [42, 23], "id": 1}`,
			res: `{"jsonrpc":"2.0","result":19,"id":1}`,
		},
		"rpc call with positional parameters 2": {
			req: `{"jsonrpc": "2.0", "method": "subtract", "params": [23, 42], "id": 2}`,
			res: `{"jsonrpc":"2.0","result":-19,"id":2}`,
		},
		"rpc call with named parameters 1": {
			req: `{"jsonrpc": "2.0", "method": "subtract", "params": {"subtrahend": 23, "minuend": 42}, "id": 3}`,
			res: `{"jsonrpc":"2.0","result":19,"id":3}`,
		},
		"rpc call with named parameters 2": {
			req: `{"jsonrpc": "2.0", "method": "subtract", "params": {"minuend": 42, "subtrahend": 23}, "id": 4}`,
			res: `{"jsonrpc":"2.0","result":19,"id":4}`,
		},
		"notif 1": {
			req: `{"jsonrpc": "2.0", "method": "update", "params": [1,2,3,4,5]}`,
			res: ``,
		},
		"notif 2": {
			req: `{"jsonrpc": "2.0", "method": "foobar"}`,
			res: ``,
		},
		"method not found": {
			req: `{"jsonrpc": "2.0", "method": "notfound", "id": "1"}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32601,"message":"Method Not Found"},"id":"1"}`,
		},
		"rpc call with invalid JSON": {
			req: `{"jsonrpc": "2.0", "method": "foobar, "params": "bar", "baz]`,
			res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"invalid character 'p' after object key:value pair"},"id":null}`,
		},
		"rpc call Batch, invalid JSON:": {
			req: `[
  {"jsonrpc": "2.0", "method": "sum", "params": [1,2,4], "id": "1"},
  {"jsonrpc": "2.0", "method"
]`,
			res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"invalid character ']' after object key"},"id":null}`,
		},
		"rpc call with an invalid Batch (but not empty)": {
			req: `[1]`,
			res: `[{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request","data":"json: cannot unmarshal number into Go value of type jsonrpc.request"},"id":null}]`,
		},
		"rpc call with invalid Batch": {
			isBatch: true,
			req:     `[1,2,3]`,
			res:     `[{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request","data":"json: cannot unmarshal number into Go value of type jsonrpc.request"},"id":null},{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request","data":"json: cannot unmarshal number into Go value of type jsonrpc.request"},"id":null},{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request","data":"json: cannot unmarshal number into Go value of type jsonrpc.request"},"id":null}]`,
		},
	}

	for desc, test := range tests {
		t.Run(desc, func(t *testing.T) {
			serverConn, clientConn := net.Pipe()
			t.Cleanup(func() {
				require.NoError(t, serverConn.Close())
				require.NoError(t, clientConn.Close())
			})

			wg := conc.NewWaitGroup()
			t.Cleanup(wg.Wait)
			wg.Go(func() {
				err := server.Handle(context.Background(), serverConn)
				require.NoError(t, err)
			})

			write(t, clientConn, test.req)

			if test.res == "" { // notification
				return
			}

			got := read(t, clientConn, len(test.res))
			if test.isBatch {
				assertBatchResponse(t, test.res, got)
			} else {
				assert.Equal(t, test.res, got)
			}
		})
	}
}

func assertBatchResponse(t *testing.T, expectedStr, actualStr string) {
	var expected []json.RawMessage
	var actual []json.RawMessage

	err := json.Unmarshal([]byte(expectedStr), &expected)
	require.NoError(t, err)
	err = json.Unmarshal([]byte(actualStr), &actual)
	require.NoError(t, err)

	assert.ElementsMatch(t, expected, actual)
}

func read(t *testing.T, c io.Reader, length int) string {
	got := make([]byte, length)
	_, err := c.Read(got)
	require.NoError(t, err)
	return string(got)
}

func write(t *testing.T, c io.Writer, data string) {
	_, err := c.Write([]byte(data))
	require.NoError(t, err)
}
