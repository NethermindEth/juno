package jsonrpc_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/go-playground/validator/v10"
	"github.com/sourcegraph/conc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
)

func TestServer_RegisterMethod(t *testing.T) {
	server := jsonrpc.NewServer(1, log.NewNopZapLogger())
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
			want:       "handler must return 2 or 3 values",
		},
		"int return": {
			handler:    func(param1, param2 int) (int, int) { return 0, 0 },
			paramNames: []jsonrpc.Parameter{{Name: "param1"}, {Name: "param2"}},
			want:       "second return value must be a *jsonrpc.Error for 2 tuple handler",
		},
		"no error return 3": {
			handler:    func(param1, param2 int) (int, int, int) { return 0, 0, 0 },
			paramNames: []jsonrpc.Parameter{{Name: "param1"}, {Name: "param2"}},
			want:       "third return value must be a *jsonrpc.Error for 3 tuple handler",
		},
		"no header return 3": {
			handler:    func(param1, param2 int) (int, int, *jsonrpc.Error) { return 0, 0, &jsonrpc.Error{} },
			paramNames: []jsonrpc.Parameter{{Name: "param1"}, {Name: "param2"}},
			want:       "second return value must be a http.Header for 3 tuple handler",
		},
		"no error return": {
			handler:    func(param1, param2 int) (any, int) { return 0, 0 },
			paramNames: []jsonrpc.Parameter{{Name: "param1"}, {Name: "param2"}},
			want:       "second return value must be a *jsonrpc.Error for 2 tuple handler",
		},
	}

	for desc, test := range tests {
		t.Run(desc, func(t *testing.T) {
			err := server.RegisterMethods(jsonrpc.Method{
				Name:    "method",
				Params:  test.paramNames,
				Handler: test.handler,
			})
			assert.EqualError(t, err, test.want, desc)
		})
	}

	t.Run("should not fail", func(t *testing.T) {
		err := server.RegisterMethods(jsonrpc.Method{
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
			Name: "acceptsContext",
			Handler: func(ctx context.Context) (int, *jsonrpc.Error) {
				require.NotNil(t, ctx)
				return 0, nil
			},
		},
		{
			Name:   "acceptsContextAndTwoParams",
			Params: []jsonrpc.Parameter{{Name: "a"}, {Name: "b"}},
			Handler: func(ctx context.Context, a, b int) (int, *jsonrpc.Error) {
				require.NotNil(t, ctx)
				return b - a, nil
			},
		},
		{
			Name:   "errorsInternally",
			Params: []jsonrpc.Parameter{},
			Handler: func() (int, *jsonrpc.Error) {
				return 0, jsonrpc.Err(jsonrpc.InternalError, nil)
			},
		},
		{
			Name:   "singleOptionalParam",
			Params: []jsonrpc.Parameter{{Name: "param", Optional: true}},
			Handler: func(param *int) (int, *jsonrpc.Error) {
				return 0, nil
			},
		},
		{
			Name: "multipleOptionalParams",
			Params: []jsonrpc.Parameter{
				{Name: "param1"},
				{Name: "param2"},
				{Name: "param3", Optional: true},
				{Name: "param4", Optional: true},
			},
			Handler: func(param1 *int, param2 []int, param3 *int, param4 []int) (int, *jsonrpc.Error) {
				return 0, nil
			},
		},
	}

	listener := CountingEventListener{}
	server := jsonrpc.NewServer(1, log.NewNopZapLogger()).
		WithValidator(validator.New()).
		WithListener(&listener)
	require.NoError(t, server.RegisterMethods(methods...))

	tests := map[string]struct {
		req                  string
		res                  string
		checkNewRequestEvent bool
		checkFailedEvent     bool
	}{
		"invalid json": {
			req: `{]`,
			res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"{]\n ^\nunexpected ']', expected a string key or '}' [line 1, position 2]"},"id":null}`,
		},
		"invalid json batch path": {
			req: `[{]`,
			res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"[{]\n  ^\nunexpected ']', expected a string key or '}' [line 1, position 3]"},"id":null}`,
		},
		"missing closing brace": {
			req: `{"jsonrpc": "2.0", "method": "method", "id": 1`,
			res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"{\"jsonrpc\": \"2.0\", \"method\": \"method\", \"id\": 1\n                                              ^\nunexpected end of input [line 1, position 47]"},"id":null}`,
		},
		"leading blank line keeps line number": {
			req: "\n{]",
			res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"\n{]\n ^\nunexpected ']', expected a string key or '}' [line 2, position 2]"},"id":null}`,
		},
		"trailing comma in object": {
			req: `{
  "jsonrpc": "2.0",
  "method": "starknet_blockNumber",
}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"{\n  \"jsonrpc\": \"2.0\",\n  \"method\": \"starknet_blockNumber\",\n}\n^\nunexpected trailing comma before '}' [line 4, position 1]"},"id":null}`,
		},
		"trailing comma with whitespace": {
			req: `{"jsonrpc": "2.0", "method": "starknet_blockNumber", }`,
			res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"{\"jsonrpc\": \"2.0\", \"method\": \"starknet_blockNumber\", }\n                                                     ^\nunexpected trailing comma before '}' [line 1, position 54]"},"id":null}`,
		},
		"empty input": {
			req: ``,
			res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"\n^\nunexpected end of input [line 1, position 1]"},"id":null}`,
		},
		"trailing comma in array": {
			req: `{
  "jsonrpc": "2.0",
  "method": "starknet_call",
  "params": ["0x1", "0x2",],
  "id": 1
}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"{\n  \"jsonrpc\": \"2.0\",\n  \"method\": \"starknet_call\",\n  \"params\": [\"0x1\", \"0x2\",],\n                          ^\nunexpected trailing comma before ']' [line 4, position 27]"},"id":null}`,
		},
		"unexpected token expecting value": {
			req: `{
  "jsonrpc": "2.0",
  "method": "starknet_chainId",
  "id": @
}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"{\n  \"jsonrpc\": \"2.0\",\n  \"method\": \"starknet_chainId\",\n  \"id\": @\n        ^\nunexpected '@', expected a value [line 4, position 9]"},"id":null}`,
		},
		"missing comma between array elements": {
			req: `{
  "jsonrpc": "2.0",
  "method": "starknet_call",
  "params": ["0x1" "0x2"],
  "id": 1
}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"{\n  \"jsonrpc\": \"2.0\",\n  \"method\": \"starknet_call\",\n  \"params\": [\"0x1\" \"0x2\"],\n                   ^\nunexpected '\"', expected ',' or ']' [line 4, position 20]"},"id":null}`,
		},
		"param type mismatch": {
			req: `{"jsonrpc": 5, "method": "x", "id": 1}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"{\"jsonrpc\": 5, \"method\": \"x\", \"id\": 1}\n            ^\nfield \"jsonrpc\" should be string, got number [line 1, position 13]"},"id":null}`,
		},
		"long line is windowed": {
			req: `{
  "jsonrpc": "2.0",
  "method": "starknet_call",
  "params": {"calldata": [` + strings.Repeat(`"0x1", `, 20) + `"0x2"] z},
  "id": 1
}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"{\n  \"jsonrpc\": \"2.0\",\n  \"method\": \"starknet_call\",\n... \"0x1\", \"0x1\", \"0x1\", \"0x1\", \"0x1\", \"0x1\", \"0x1\", \"0x1\", \"0x1\", \"0x2\"] z},\n                                                                          ^\nunexpected 'z', expected ',' or '}' [line 4, position 174]"},"id":null}`,
		},
		"error at start of long line": {
			req: `{@"jsonrpc": "2.0", "method": "starknet_estimateFee", "params": [` + strings.Repeat(`"0xdeadbeef", `, 20) + `"0x0"]}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"{@\"jsonrpc\": \"2.0\", \"method\": \"starknet_estimateFee\", \"params\": [\"0xdeadbe...\n ^\nunexpected '@', expected a string key or '}' [line 1, position 2]"},"id":null}`,
		},
		"top-level type mismatch": {
			req: `5`,
			res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"5\n^\nexpected a JSON object, got number [line 1, position 1]"},"id":null}`,
		},
		"untranslatable syntax error": {
			req: `{
  "jsonrpc": "2.0",
  "method": "starknet_syncing",
  "params": truX
}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"{\n  \"jsonrpc\": \"2.0\",\n  \"method\": \"starknet_syncing\",\n  \"params\": truX\n               ^\ninvalid character 'X' in literal true (expecting 'e') [line 4, position 16]"},"id":null}`,
		},
		"error on a middle line": {
			req: "{\n\"a\" 1\n}",
			res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"{\n\"a\" 1\n    ^\nunexpected '1', expected ':' [line 2, position 5]"},"id":null}`,
		},
		"multiline starknet request with missing comma": {
			req: `{
  "jsonrpc": "2.0",
  "method": "starknet_getStorageAt",
  "params": ["0x4c5772d", "0x206f38f" "latest"],
  "id": 1
}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"{\n  \"jsonrpc\": \"2.0\",\n  \"method\": \"starknet_getStorageAt\",\n  \"params\": [\"0x4c5772d\", \"0x206f38f\" \"latest\"],\n                                      ^\nunexpected '\"', expected ',' or ']' [line 4, position 39]"},"id":null}`,
		},
		"error past the captured window": {
			req: "[\n" + strings.Repeat("\"0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7\",\n", 40) + "\"0xbad\" \"0x1\"\n]",
			res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"\"0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7\",\n\"0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7\",\n\"0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7\",\n\"0xbad\" \"0x1\"\n        ^\nunexpected '\"', expected ',' or ']' [line 42, position 9]"},"id":null}`,
		},
		"context is capped at three lines within the window": {
			req: `{
  "jsonrpc": "2.0",
  "method": "starknet_call",
  "params": {
    "contract_address": "0x04c5772d",
    "entry_point_selector": "0x0206f38f"
    "calldata": []
  },
  "id": 1
}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"  \"params\": {\n    \"contract_address\": \"0x04c5772d\",\n    \"entry_point_selector\": \"0x0206f38f\"\n    \"calldata\": []\n    ^\nunexpected '\"', expected ',' or '}' [line 7, position 5]"},"id":null}`,
		},
		"long line is windowed on both sides": {
			req: `{"jsonrpc": "2.0", "method": "starknet_getStorageAt", "params": {"contract_address": "0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7" @ "key": "0x02f0b3c5710379609eb5495f1ecd348cb28167711b73609fe565a72734550354"}, "id": 1}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"...84644ddd6b96f7c741b1562b82f9e004dc7\" @ \"key\": \"0x02f0b3c5710379609eb5495f1...\n                                        ^\nunexpected '@', expected ',' or '}' [line 1, position 155]"},"id":null}`,
		},
		"long preceding context line is windowed": {
			req: `{
  "jsonrpc": "2.0",
  "method": "starknet_call",
  "params": ["0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x02f0b3c5710379609eb5495f1ecd348cb28167711b73609fe565a72734550354", "latest"]
  "id": 1
}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"  \"jsonrpc\": \"2.0\",\n  \"method\": \"starknet_call\",\n  \"params\": [\"0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e...\n  \"id\": 1\n  ^\nunexpected '\"', expected ',' or '}' [line 5, position 3]"},"id":null}`,
		},
		"oversized line drops all preceding context": {
			req: `{
  "jsonrpc": "2.0",
  "method": "starknet_estimateFee",
  "params": {"request": [{"calldata": [` + strings.Repeat(`"0xdeadbeef", `, 40) + `"0x0"]}]}
  "id": 1
}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"  \"id\": 1\n  ^\nunexpected '\"', expected ',' or '}' [line 5, position 3]"},"id":null}`,
		},
		"column counts runes not bytes": {
			req: `{
  "jsonrpc": "2.0",
  "method": "starknet_call",
  "👍": @
}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"{\n  \"jsonrpc\": \"2.0\",\n  \"method\": \"starknet_call\",\n  \"👍\": @\n       ^\nunexpected '@', expected a value [line 4, position 8]"},"id":null}`,
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
			res: `{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params","data":"missing required params: num"},"id":5}`,
		},
		"too many params": {
			req: `{"jsonrpc" : "2.0", "method" : "method", "params" : [3, false, "error message", "too many"] , "id" : 3}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params","data":"expected between 1 and 3 params, got 4"},"id":3}`,
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
			res: `{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params","data":"missing non-optional param: num"},"id":22}`,
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
			req: `[{"jsonrpc" : "2.0", "method" : "method",
					"params" : { "num" : 5 } , "id" : 5},
					{"jsonrpc" : "2.0", "method" : "method",
					"params" : { "num" : 44 } , "id" : 6}]`,
			res: `[{"jsonrpc":"2.0","result":{"doubled":10},"id":5},{"jsonrpc":"2.0","result":{"doubled":88},"id":6}]`,
		},
		"failing and successful requests mixed in a batch": {
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
			res: `[{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request","data":"json: cannot unmarshal array into Go value of type jsonrpc.Request"},"id":null},{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request","data":"json: cannot unmarshal array into Go value of type jsonrpc.Request"},"id":null}]`,
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
			req:                  `{"jsonrpc" : "2.0", "method" : "validation", "params" : [{"A": 1}], "id" : 1}`,
			res:                  `{"jsonrpc":"2.0","result":1,"id":1}`,
			checkNewRequestEvent: true,
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
		"handler accepts context without params": {
			req: `{"jsonrpc": "2.0", "method": "acceptsContext","id": 1}`,
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
			res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"{\"jsonrpc\": \"2.0\", \"method\": \"foobar, \"params\": \"bar\", \"baz]\n                                       ^\nunexpected 'p', expected ',' or '}' [line 1, position 40]"},"id":null}`,
		},
		"rpc call Batch, invalid JSON:": {
			req: `[
  {"jsonrpc": "2.0", "method": "sum", "params": [1,2,4], "id": "1"},
  {"jsonrpc": "2.0", "method"
]`,
			res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"[\n  {\"jsonrpc\": \"2.0\", \"method\": \"sum\", \"params\": [1,2,4], \"id\": \"1\"},\n  {\"jsonrpc\": \"2.0\", \"method\"\n]\n^\nunexpected ']', expected ':' [line 4, position 1]"},"id":null}`,
		},
		"rpc call with an invalid Batch (but not empty)": {
			req: `[1]`,
			res: `[{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request","data":"json: cannot unmarshal number into Go value of type jsonrpc.Request"},"id":null}]`,
		},
		"rpc call with invalid Batch": {
			req: `[1,2,3]`,
			res: `[{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request","data":"json: cannot unmarshal number into Go value of type jsonrpc.Request"},"id":null},{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request","data":"json: cannot unmarshal number into Go value of type jsonrpc.Request"},"id":null},{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request","data":"json: cannot unmarshal number into Go value of type jsonrpc.Request"},"id":null}]`,
		},
		"fails internally": {
			req:              `{"jsonrpc": "2.0", "method": "errorsInternally", "params": {}, "id": 1}`,
			res:              `{"jsonrpc":"2.0","error":{"code":-32603,"message":"Internal error"},"id":1}`,
			checkFailedEvent: true,
		},
		"empty optional param": {
			req: `{"jsonrpc": "2.0", "method": "singleOptionalParam", "params": {}, "id": 1}`,
			res: `{"jsonrpc":"2.0","result":0,"id":1}`,
		},
		"null optional param": {
			req: `{"jsonrpc": "2.0", "method": "singleOptionalParam", "id": 1}`,
			res: `{"jsonrpc":"2.0","result":0,"id":1}`,
		},
		"empty multiple optional params": {
			req: `{"jsonrpc": "2.0", "method": "multipleOptionalParams", "params": {"param1": 1, "param2": [2, 3]}, "id": 1}`,
			res: `{"jsonrpc":"2.0","result":0,"id":1}`,
		},
		"empty multiple optional positional params": {
			req: `{"jsonrpc": "2.0", "method": "multipleOptionalParams", "params": [1, [2, 3]], "id": 1}`,
			res: `{"jsonrpc":"2.0","result":0,"id":1}`,
		},
		"junk + valid params": {
			req: `{"jsonrpc": "2.0", "method": "multipleOptionalParams", "params": {"param1": 1, "param2": [2, 3], "junk": "junk"}, "id": 1}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params","data":"unexpected params: junk"},"id":1}`,
		},
	}

	for desc, test := range tests {
		t.Run(desc, func(t *testing.T) {
			oldNewRequestEventCount := len(listener.OnNewRequestLogs)
			oldRequestFailedEventCount := len(listener.OnRequestFailedCalls)
			oldRequestHandledCalls := len(listener.OnRequestHandledCalls)

			res, httpHeader, err := server.HandleReader(t.Context(), strings.NewReader(test.req))
			require.NoError(t, err)
			assert.NotNil(t, httpHeader)

			// tests that have an empty response cannot be parsed into a json
			if test.res != "" {
				assert.JSONEq(t, test.res, string(res))
			} else {
				assert.Equal(t, test.res, string(res))
			}

			if test.checkNewRequestEvent {
				assert.Greater(t, len(listener.OnNewRequestLogs), oldNewRequestEventCount)
				if !test.checkFailedEvent {
					assert.Greater(t, len(listener.OnRequestHandledCalls), oldRequestHandledCalls)
				}
			}
			if test.checkFailedEvent {
				assert.Greater(t, len(listener.OnRequestFailedCalls), oldRequestFailedEventCount)
			}
		})
	}
}

func TestServerWithDisabledBatchRequests(t *testing.T) {
	server := jsonrpc.NewServer(1, log.NewNopZapLogger())

	err := server.RegisterMethods(
		jsonrpc.Method{
			Name:   "identity",
			Params: []jsonrpc.Parameter{{Name: "num"}},
			Handler: func(num *int) (int, *jsonrpc.Error) {
				return *num, nil
			},
		},
	)
	require.NoError(t, err)

	// batch requests work by default
	const batchedRequest = `[
		{
			"jsonrpc": "2.0",
			"method": "identity",
			"params": { "num": 3 },
			"id": 1
		},
		{
			"jsonrpc": "2.0",
			"method": "identity",
			"params": { "num": 7 },
			"id": 2
		}
	]`
	const batchedResponse = `[
		{
			"jsonrpc": "2.0",
			"result": 3,
			"id": 1
		},
		{
			"jsonrpc": "2.0",
			"result": 7,
			"id": 2
		}
	]`
	resp, _, err := server.HandleReader(t.Context(), strings.NewReader(batchedRequest))
	require.NoError(t, err)
	assert.JSONEq(t, batchedResponse, string(resp))

	// batch requests stop working when explicitly disabled
	server.DisableBatchRequests(true)
	resp, _, err = server.HandleReader(t.Context(), strings.NewReader(batchedRequest))
	require.NoError(t, err)

	const invalidResponse = `{
		"jsonrpc": "2.0",
		"error": {
			"code": -32600,
			"message": "Invalid Request",
			"data": "batch requests are disabled"
		},
		"id": null
	}`
	assert.JSONEq(t, invalidResponse, string(resp))
}

var benchHandleR http.Header

func BenchmarkHandle(b *testing.B) {
	listener := CountingEventListener{}
	server := jsonrpc.NewServer(1, log.NewNopZapLogger()).
		WithValidator(validator.New()).
		WithListener(&listener)
	require.NoError(b, server.RegisterMethods(jsonrpc.Method{
		Name:    "bench",
		Handler: func() (int, *jsonrpc.Error) { return 0, nil },
	}))

	const request = `{"jsonrpc":"2.0","id":1,"method":"test"}`
	var header http.Header
	var err error

	for b.Loop() {
		_, header, err = server.HandleReader(b.Context(), strings.NewReader(request))
		require.NoError(b, err)
		require.NotNil(b, header)
	}
	benchHandleR = header
}

// BenchmarkHandleLargeRequest measures large bodies (up to the 10MB request
// limit), where the TeeReader copy scales with the input size.
func BenchmarkHandleLargeRequest(b *testing.B) {
	server := jsonrpc.NewServer(1, log.NewNopZapLogger()).WithValidator(validator.New())
	require.NoError(b, server.RegisterMethods(jsonrpc.Method{
		Name:    "echo",
		Params:  []jsonrpc.Parameter{{Name: "data"}},
		Handler: func(data string) (int, *jsonrpc.Error) { return len(data), nil },
	}))

	sizes := []struct {
		name  string
		bytes int
	}{
		{"1MB", 1 << 20},
		{"10MB", 10 << 20},
	}
	for _, size := range sizes {
		b.Run(size.name, func(b *testing.B) {
			request := `{"jsonrpc":"2.0","method":"echo","params":["` + strings.Repeat("a", size.bytes) + `"],"id":1}`
			b.SetBytes(int64(len(request)))
			b.ResetTimer()
			var err error
			for b.Loop() {
				_, _, err = server.HandleReader(b.Context(), strings.NewReader(request))
				require.NoError(b, err)
			}
		})
	}
}

func TestCannotWriteToConnInHandler(t *testing.T) {
	server := jsonrpc.NewServer(1, log.NewNopZapLogger())
	require.NoError(t, server.RegisterMethods(jsonrpc.Method{
		Name: "test",
		Handler: func(ctx context.Context) (int, *jsonrpc.Error) {
			w, ok := jsonrpc.ConnFromContext(ctx)
			require.False(t, ok)
			require.Nil(t, w)
			return 0, nil
		},
	}))
	res, header, err := server.HandleReader(t.Context(), strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"test"}`))
	require.NoError(t, err)
	require.Equal(t, `{"jsonrpc":"2.0","result":0,"id":1}`, string(res))
	require.NotNil(t, header)
}

type fakeConn struct{}

func (fc *fakeConn) Write(p []byte) (int, error) {
	return 0, nil
}

func (fc *fakeConn) Equal(other jsonrpc.Conn) bool {
	return false
}

func TestWriteToConnInHandler(t *testing.T) {
	testBytes := "written from handler"
	server := jsonrpc.NewServer(1, log.NewNopZapLogger())
	wg := conc.NewWaitGroup()
	t.Cleanup(wg.Wait)
	require.NoError(t, server.RegisterMethods(jsonrpc.Method{
		Name: "test",
		Handler: func(ctx context.Context) (int, *jsonrpc.Error) {
			w, ok := jsonrpc.ConnFromContext(ctx)
			require.True(t, ok)
			wg.Go(func() {
				n, err := w.Write([]byte(testBytes))
				require.NoError(t, err)
				require.Equal(t, len(testBytes), n)
				require.True(t, w.Equal(w)) //nolint:gocritic
				require.False(t, w.Equal(&fakeConn{}))
			})
			return 0, nil
		},
	}))

	serverConn, clientConn := net.Pipe()
	t.Cleanup(func() {
		require.NoError(t, serverConn.Close())
		require.NoError(t, clientConn.Close())
	})

	wg.Go(func() {
		err := server.HandleReadWriter(t.Context(), serverConn)
		require.NoError(t, err)
	})

	write(t, clientConn, `{"jsonrpc":"2.0","id":1,"method":"test","params":[]}`)
	initialResp := `{"jsonrpc":"2.0","result":0,"id":1}`
	require.Equal(t, initialResp, read(t, clientConn, len(initialResp)))
	require.Equal(t, testBytes, read(t, clientConn, len(testBytes)))
}

func TestWriteToClosedConnInHandler(t *testing.T) {
	server := jsonrpc.NewServer(1, log.NewNopZapLogger())
	wg := conc.NewWaitGroup()
	t.Cleanup(wg.Wait)
	require.NoError(t, server.RegisterMethods(jsonrpc.Method{
		Name: "test",
		Handler: func(ctx context.Context) (int, *jsonrpc.Error) {
			w, ok := jsonrpc.ConnFromContext(ctx)
			require.True(t, ok)
			wg.Go(func() {
				for range 3 {
					_, err := w.Write([]byte("test"))
					require.ErrorIs(t, err, io.ErrClosedPipe)
					require.ErrorContains(t, err, "there was an error while writing the initial response")
				}
			})
			return 0, nil
		},
	}))

	serverConn, clientConn := net.Pipe()
	t.Cleanup(func() {
		require.NoError(t, serverConn.Close())
		// We close clientConn early.
	})

	wg.Go(func() {
		err := server.HandleReadWriter(t.Context(), serverConn)
		require.ErrorIs(t, err, io.ErrClosedPipe)
	})

	write(t, clientConn, `{"jsonrpc":"2.0","id":1,"method":"test","params":[]}`)
	require.NoError(t, clientConn.Close())
}

func TestRequest_MarshalLogObject(t *testing.T) {
	tests := map[string]struct {
		req  *jsonrpc.Request
		want map[string]any
	}{
		"full request": {
			req: &jsonrpc.Request{
				Version: "2.0",
				Method:  "starknet_getBlockWithTxs",
				Params:  []any{"latest"},
				ID:      1,
			},
			want: map[string]any{
				"jsonrpc": "2.0",
				"method":  "starknet_getBlockWithTxs",
				"id":      1,
				"params":  []any{"latest"},
			},
		},
		"nil id and params omitted": {
			req: &jsonrpc.Request{
				Version: "2.0",
				Method:  "ping",
			},
			want: map[string]any{
				"jsonrpc": "2.0",
				"method":  "ping",
			},
		},
		"string id": {
			req: &jsonrpc.Request{
				Version: "2.0",
				Method:  "ping",
				ID:      "abc",
			},
			want: map[string]any{
				"jsonrpc": "2.0",
				"method":  "ping",
				"id":      "abc",
			},
		},
		"method with control chars is sanitised": {
			req: &jsonrpc.Request{
				Version: "2.0",
				Method:  "evil\nINJECTED\r\tmethod",
			},
			want: map[string]any{
				"jsonrpc": "2.0",
				"method":  "evilINJECTED\tmethod",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			enc := zapcore.NewMapObjectEncoder()
			require.NoError(t, tc.req.MarshalLogObject(enc))
			assert.Equal(t, tc.want, enc.Fields)
		})
	}
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
