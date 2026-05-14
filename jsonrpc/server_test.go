package jsonrpc_test

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc"
	"github.com/NethermindEth/juno/utils/jsonx"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/go-playground/validator/v10"
	"github.com/sourcegraph/conc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
)

// assertResponseIgnoringErrorData compares two JSON-RPC responses, treating
// `error.data` as opaque — it must be present and non-empty in the actual
// response, but its text is not compared. Sonic's parse-error phrasing
// differs across CPU architectures (AMD64 native asm path vs ARM64 compat
// fallback), so any test whose `error.data` carries encoder-specific text
// must use this helper rather than asserting the exact bytes.
func assertResponseIgnoringErrorData(t *testing.T, expected, actual string) {
	t.Helper()
	strip := func(s string) any {
		var v any
		require.NoError(t, jsonx.Unmarshal([]byte(s), &v))
		stripData(v)
		return v
	}
	assert.Equal(t, strip(expected), strip(actual))
	assert.True(t, hasNonEmptyErrorData([]byte(actual)),
		"error.data must be present and non-empty in actual response")
}

// stripData removes `error.data` fields from the decoded JSON value so the
// rest of the structure can be compared exactly.
func stripData(v any) {
	switch x := v.(type) {
	case map[string]any:
		if errObj, ok := x["error"].(map[string]any); ok {
			delete(errObj, "data")
		}
		for _, child := range x {
			stripData(child)
		}
	case []any:
		for _, child := range x {
			stripData(child)
		}
	}
}

// hasNonEmptyErrorData reports whether every `error` object in the parsed
// JSON carries a non-empty `data` field. Guards against encoders that
// silently drop the field.
func hasNonEmptyErrorData(raw []byte) bool {
	var v any
	if err := jsonx.Unmarshal(raw, &v); err != nil {
		return false
	}
	return checkErrorData(v)
}

func checkErrorData(v any) bool {
	switch x := v.(type) {
	case map[string]any:
		if errObj, ok := x["error"].(map[string]any); ok {
			s, _ := errObj["data"].(string)
			if s == "" {
				return false
			}
		}
		for _, child := range x {
			if !checkErrorData(child) {
				return false
			}
		}
	case []any:
		for _, child := range x {
			if !checkErrorData(child) {
				return false
			}
		}
	}
	return true
}

type testMethodParams struct {
	Num         *int `json:"num"`
	ShouldError bool `json:"shouldError,omitempty"`
	Msg         any  `json:"msg,omitempty"`
}

type testSubtractParams struct {
	Minuend    int `json:"minuend"`
	Subtrahend int `json:"subtrahend"`
}

type testUpdateParams struct {
	A int `json:"a"`
	B int `json:"b"`
	C int `json:"c"`
	D int `json:"d"`
	E int `json:"e"`
}

type testValidationStruct struct {
	A int `validate:"min=1"`
}

type testValidationParams struct {
	Param testValidationStruct `json:"param"`
}
type testValidationSliceParams struct {
	Param []testValidationStruct `json:"param" validate:"dive"`
}
type testValidationPointerParams struct {
	Param *testValidationStruct `json:"param"`
}
type testValidationMapPointerParams struct {
	Param map[string]*testValidationStruct `json:"param" validate:"dive"`
}

type testAcceptsContextAndTwoParamsParams struct {
	A int `json:"a"`
	B int `json:"b"`
}

type testSingleOptionalParam struct {
	Param *int `json:"param,omitempty"`
}

type testMultipleOptionalParams struct {
	Param1 *int  `json:"param1"`
	Param2 []int `json:"param2"`
	Param3 *int  `json:"param3,omitempty"`
	Param4 []int `json:"param4,omitempty"`
}

func TestHandle(t *testing.T) {
	methods := []jsonrpc.Method{
		jsonrpc.Register("method", func(p *testMethodParams) (any, *jsonrpc.Error) {
			if p.ShouldError {
				return nil, &jsonrpc.Error{Code: 44, Message: "Expected Error", Data: p.Msg}
			}
			return struct {
				Doubled int `json:"doubled"`
			}{*p.Num * 2}, nil
		}),
		jsonrpc.Register("subtract", func(p *testSubtractParams) (int, *jsonrpc.Error) {
			return p.Minuend - p.Subtrahend, nil
		}),
		jsonrpc.Register("update", func(_ *testUpdateParams) (int, *jsonrpc.Error) {
			return 0, nil
		}),
		jsonrpc.Register("foobar", func(_ *jsonrpc.NoParams) (int, *jsonrpc.Error) {
			return 0, nil
		}),
		jsonrpc.Register("validation", func(p *testValidationParams) (int, *jsonrpc.Error) {
			return p.Param.A, nil
		}),
		jsonrpc.Register("validationSlice", func(p *testValidationSliceParams) (int, *jsonrpc.Error) {
			return p.Param[0].A, nil
		}),
		jsonrpc.Register("validationPointer", func(p *testValidationPointerParams) (int, *jsonrpc.Error) {
			return p.Param.A, nil
		}),
		jsonrpc.Register("validationMapPointer",
			func(p *testValidationMapPointerParams) (int, *jsonrpc.Error) {
				return p.Param["expectedkey"].A, nil
			}),
		jsonrpc.RegisterC("acceptsContext",
			func(ctx context.Context, _ *jsonrpc.NoParams) (int, *jsonrpc.Error) {
				require.NotNil(t, ctx)
				return 0, nil
			}),
		jsonrpc.RegisterC("acceptsContextAndTwoParams",
			func(ctx context.Context, p *testAcceptsContextAndTwoParamsParams) (int, *jsonrpc.Error) {
				require.NotNil(t, ctx)
				return p.B - p.A, nil
			}),
		jsonrpc.Register("errorsInternally", func(_ *jsonrpc.NoParams) (int, *jsonrpc.Error) {
			return 0, jsonrpc.Err(jsonrpc.InternalError, nil)
		}),
		jsonrpc.Register("singleOptionalParam", func(_ *testSingleOptionalParam) (int, *jsonrpc.Error) {
			return 0, nil
		}),
		jsonrpc.Register("multipleOptionalParams",
			func(_ *testMultipleOptionalParams) (int, *jsonrpc.Error) {
				return 0, nil
			}),
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
		// ignoreErrorData set when the JSON encoder's parse-error text is
		// surfaced in `error.data`. Sonic's text differs across CPU
		// architectures (AMD64 native vs ARM64 compat path), so we only
		// assert that the field exists and is non-empty.
		ignoreErrorData bool
	}{
		"invalid json": {
			req: `{]`,
			//nolint:lll // We can't break the json value
			res:             `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"placeholder"},"id":null}`,
			ignoreErrorData: true,
		},
		"invalid json batch path": {
			req: `[{]`,
			//nolint:lll // We can't break the json value
			res:             `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"placeholder"},"id":null}`,
			ignoreErrorData: true,
		},
		"wrong version": {
			req: `{"jsonrpc" : "1.0", "id" : 1}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request",
			"data":"unsupported RPC request version"},"id":1}`,
		},
		"wrong version with null id": {
			req: `{"jsonrpc" : "1.0", "id" : null}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request",
			"data":"unsupported RPC request version"},"id":null}`,
		},
		"non existent method": {
			req: `{"jsonrpc" : "2.0", "method" : "doesnotexits" , "id" : 2}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32601,"message":"Method Not Found"},"id":2}`,
		},
		"no params": {
			req: `{"jsonrpc" : "2.0", "method" : "method", "id" : 5}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params",
			"data":"missing non-optional param field"},"id":5}`,
		},
		"too many params": {
			req: `{"jsonrpc" : "2.0", "method" : "method", 
				"params" : [3, false, "error message", "too many"] , "id" : 3}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params",
			"data":"missing/unexpected params in list"},"id":3}`,
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
			//nolint:lll // We can't break the json value
			res:             `[{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request","data":"placeholder"},"id":null},{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request","data":"placeholder"},"id":null}]`,
			ignoreErrorData: true,
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
			//nolint:lll // We can't break the json value
			req: `{"jsonrpc" : "2.0", "method" : "method", "params" : ["3", false, "error message"] , "id" : 3}`,
			//nolint:lll // We can't break the json value
			res:             `{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params","data":"placeholder"},"id":3}`,
			ignoreErrorData: true,
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
			res: `{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params",` +
				`"data":"Key: 'testValidationParams.Param.A'` +
				` Error:Field validation for 'A' failed on the 'min' tag"},"id":1}`,
		},
		"valid value in struct": {
			req:                  `{"jsonrpc" : "2.0", "method" : "validation", "params" : [{"A": 1}], "id" : 1}`,
			res:                  `{"jsonrpc":"2.0","result":1,"id":1}`,
			checkNewRequestEvent: true,
		},
		"invalid value in struct pointer": {
			req: `{"jsonrpc" : "2.0", "method" : "validationPointer", "params" : [ {"A": 0} ], "id" : 1}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params",` +
				`"data":"Key: 'testValidationPointerParams.Param.A'` +
				` Error:Field validation for 'A' failed on the 'min' tag"},"id":1}`,
		},
		"valid value in struct pointer": {
			req: `{"jsonrpc" : "2.0", "method" : "validationPointer", "params" : [ {"A": 1} ], "id" : 1}`,
			res: `{"jsonrpc":"2.0","result":1,"id":1}`,
		},
		"invalid value in slice struct": {
			req: `{"jsonrpc" : "2.0", "method" : "validationSlice", "params" : [ [{"A": 0}] ], "id" : 1}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params",` +
				`"data":"Key: 'testValidationSliceParams.Param[0].A'` +
				` Error:Field validation for 'A' failed on the 'min' tag"},"id":1}`,
		},
		"valid value in slice of struct": {
			req: `{"jsonrpc" : "2.0", "method" : "validationSlice", "params" : [[{"A": 1}]], "id" : 1}`,
			res: `{"jsonrpc":"2.0","result":1,"id":1}`,
		},
		"invalid value in map of pointer": {
			req: `{"jsonrpc" : "2.0", "method" : "validationMapPointer",` +
				` "params" : [ { "notthexpectedkey" : {"A": 0}} ], "id" : 1}`,
			res: `{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params",` +
				`"data":"Key: 'testValidationMapPointerParams.Param[notthexpectedkey].A'` +
				` Error:Field validation for 'A' failed on the 'min' tag"},"id":1}`,
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
			//nolint:lll // We can't break the json value
			res:             `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"placeholder"},"id":null}`,
			ignoreErrorData: true,
		},
		"rpc call Batch, invalid JSON:": {
			req: `[
  {"jsonrpc": "2.0", "method": "sum", "params": [1,2,4], "id": "1"},
  {"jsonrpc": "2.0", "method"
]`,
			//nolint:lll // We can't break the json value
			res:             `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"placeholder"},"id":null}`,
			ignoreErrorData: true,
		},
		"rpc call with an invalid Batch (but not empty)": {
			req: `[1]`,
			//nolint:lll // We can't break the json value
			res:             `[{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request","data":"placeholder"},"id":null}]`,
			ignoreErrorData: true,
		},
		"rpc call with invalid Batch": {
			req: `[1,2,3]`,
			//nolint:lll // We can't break the json value
			res:             `[{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request","data":"placeholder"},"id":null},{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request","data":"placeholder"},"id":null},{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request","data":"placeholder"},"id":null}]`,
			ignoreErrorData: true,
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
			//nolint:lll // We can't break the json value
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
				if test.ignoreErrorData {
					assertResponseIgnoringErrorData(t, test.res, string(res))
				} else {
					assert.JSONEq(t, test.res, string(res))
				}
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

type identityParams struct {
	Num *int `json:"num"`
}

func TestServerWithDisabledBatchRequests(t *testing.T) {
	server := jsonrpc.NewServer(1, log.NewNopZapLogger())

	err := server.RegisterMethods(
		jsonrpc.Register("identity",
			func(p *identityParams) (int, *jsonrpc.Error) { return *p.Num, nil }),
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
	require.NoError(b, server.RegisterMethods(jsonrpc.Register("bench",
		func(_ *jsonrpc.NoParams) (int, *jsonrpc.Error) { return 0, nil })))

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

func TestCannotWriteToConnInHandler(t *testing.T) {
	server := jsonrpc.NewServer(1, log.NewNopZapLogger())
	require.NoError(t, server.RegisterMethods(jsonrpc.RegisterC("test",
		func(ctx context.Context, _ *jsonrpc.NoParams) (int, *jsonrpc.Error) {
			w, ok := jsonrpc.ConnFromContext(ctx)
			require.False(t, ok)
			require.Nil(t, w)
			return 0, nil
		})))
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
	require.NoError(t, server.RegisterMethods(jsonrpc.RegisterC("test",
		func(ctx context.Context, _ *jsonrpc.NoParams) (int, *jsonrpc.Error) {
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
		})))

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
	require.NoError(t, server.RegisterMethods(jsonrpc.RegisterC("test",
		func(ctx context.Context, _ *jsonrpc.NoParams) (int, *jsonrpc.Error) {
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
		})))

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
				Params:  json.RawMessage(`["latest"]`),
				ID:      1,
			},
			want: map[string]any{
				"jsonrpc": "2.0",
				"method":  "starknet_getBlockWithTxs",
				"id":      1,
				"params":  json.RawMessage(`["latest"]`),
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

// TestPretouchProductionHandlers registers Juno's real RPC methods
// (v8, v9, v10) on jsonrpc.Server instances and dumps the resulting
// PretouchedTypes list. The list is the actual set of Go types that
// sonic compiled encode/decode paths for at server startup, with no
// placeholder/stub handlers.
//
// Network and storage interfaces (bcReader, syncReader, vm) are passed
// nil — those are only touched when handlers run, not at registration.
func TestPretouchProductionHandlers(t *testing.T) {
	logger := log.NewNopZapLogger()
	handler := rpc.New(nil, nil, nil, "test", logger, &networks.Network{})

	cases := []struct {
		name    string
		methods func() ([]jsonrpc.Method, string)
	}{
		{"v0.10", handler.MethodsV0_10},
		{"v0.9", handler.MethodsV0_9},
		{"v0.8", handler.MethodsV0_8},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			methods, _ := tc.methods()
			server := jsonrpc.NewServer(1, logger)
			require.NoError(t, server.RegisterMethods(methods...))

			types := server.PretouchedTypes()
			names := make([]string, len(types))
			for i, ty := range types {
				names[i] = ty.String()
			}
			sort.Strings(names)

			t.Logf("%s: %d unique types pretouched", tc.name, len(names))
			for _, n := range names {
				t.Logf("  %s", n)
			}

			// Sanity: envelope must be there.
			require.Contains(t, names, "jsonrpc.Request")
			require.Contains(t, names, "jsonrpc.response")
			require.Contains(t, names, "jsonrpc.Error")
			// And we registered something beyond the envelope-only set.
			require.Greater(t, len(names), 3)
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
