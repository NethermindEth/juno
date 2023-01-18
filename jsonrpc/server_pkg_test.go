package jsonrpc

import (
	"bufio"
	"bytes"
	"io"
	"reflect"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/assert"
)

func TestIsBatch(t *testing.T) {
	tests := []struct {
		req   string
		batch bool
		left  string
	}{
		{
			req:   "{}",
			batch: false,
			left:  "{}",
		},
		{
			req:   " \r\n\t{}",
			batch: false,
			left:  "{}",
		},
		{
			req:   "[{},{}]",
			batch: true,
			left:  "[{},{}]",
		},
		{
			req:   " \r\n\t[{},{}]",
			batch: true,
			left:  "[{},{}]",
		},
	}

	for _, test := range tests {
		reader := bytes.NewReader([]byte(test.req))
		bufferedReader := bufio.NewReader(reader)
		assert.Equal(t, test.batch, isBatch(bufferedReader), test.req)

		left, err := io.ReadAll(bufferedReader)
		assert.NoError(t, err)
		assert.Equal(t, test.left, string(left))
	}
}

func TestRequest(t *testing.T) {
	failingTests := map[string]struct {
		data []byte
		want string
	}{
		"empty req":    {[]byte(""), "EOF"},
		"empty object": {[]byte("{}"), "unsupported RPC request version"},
		"wrong version": {[]byte(`
				{
					"jsonrpc" : "1.0"
				}`), "unsupported RPC request version"},
		"no method": {[]byte(`
				{
					"jsonrpc" : "2.0"
				}`), "no method specified"},
		"number param": {[]byte(`
				{
					"jsonrpc" : "2.0",
					"method" : "rpc_call",
					"params" : 44
				}`), "params should be an array or an object"},
		"string param": {[]byte(`
				{
					"jsonrpc" : "2.0",
					"method" : "rpc_call",
					"params" : "44"
				}`), "params should be an array or an object"},
		"array id": {[]byte(`
				{
					"jsonrpc" : "2.0",
					"method" : "rpc_call",
					"params" : { "malatya" : "44"},
					"id"     : [37]
				}`), "id should be a string or an integer"},
		"map id": {[]byte(`
				{
					"jsonrpc" : "2.0",
					"method" : "rpc_call",
					"params" : { "malatya" : "44"},
					"id"     : { "44" : "37"}
				}`), "id should be a string or an integer"},
		"float id": {[]byte(`
				{
					"jsonrpc" : "2.0",
					"method" : "rpc_call",
					"params" : { "malatya" : "44"},
					"id"     : 44.37
				}`), "id should be a string or an integer"},
	}

	for desc, test := range failingTests {
		_, err := newRequest(bytes.NewReader(test.data))
		assert.EqualError(t, err, test.want, desc)
	}

	happyPathTests := map[string]struct {
		data []byte
	}{
		"uint id": {[]byte(`
				{
					"jsonrpc" : "2.0",
					"method" : "rpc_call",
					"params" : { "malatya" : "44"},
					"id"     : 44
				}`)},
		"int id": {[]byte(`
				{
					"jsonrpc" : "2.0",
					"method" : "rpc_call",
					"params" : { "malatya" : "44"},
					"id"     : -44
				}`)},
		"string id": {[]byte(`
				{
					"jsonrpc" : "2.0",
					"method" : "rpc_call",
					"params" : { "malatya" : 44, "depdep" : { "plate" : "37" }},
					"id"     : "-44"
				}`)},
	}

	for desc, test := range happyPathTests {
		_, err := newRequest(bytes.NewReader(test.data))
		assert.NoError(t, err, desc)
	}
}

func TestBuildArguments(t *testing.T) {
	type arg struct {
		Val1 int        `json:"val_1"`
		Val2 string     `json:"val_2"`
		Val3 *felt.Felt `json:"val_3"`
	}

	paramNames := []Parameter{{Name: "param1"}, {Name: "param2", Optional: true}}
	felt, _ := new(felt.Felt).SetString("0x4437")

	tests := []struct {
		handler    any
		req        string
		shouldFail bool
		errorMsg   string
	}{
		{
			handler: func(param1 int, param2 *arg) (any, error) {
				assert.Equal(t, 44, param1)
				assert.Equal(t, true, param2 == nil)
				return 0, nil
			},
			req: `{
					"jsonrpc" : "2.0",
					"method" : "method",
					"params" : { 
						"param1" : 44 
					},
					"id"     : 44
				}`,
		},
		{
			handler: func(param1 int, param2 *arg) (any, error) {
				assert.Equal(t, 44, param1)
				assert.Equal(t, arg{
					Val1: 37,
					Val2: "juno",
					Val3: felt,
				}, *param2)
				return 0, nil
			},
			req: `{
					"jsonrpc" : "2.0",
					"method" : "method",
					"params" : { 
						"param1" : 44, 
						"param2" : { 
							"val_1" : 37,
							"val_2" : "juno",
							"val_3" : "0x4437"
						}
					},
					"id"     : 44
				}`,
		},
		{
			handler: func(param1 int, param2 arg) (any, error) {
				assert.Equal(t, 44, param1)
				assert.Equal(t, arg{
					Val1: 37,
					Val2: "juno",
					Val3: felt,
				}, param2)
				return 0, nil
			},
			req: `{
					"jsonrpc" : "2.0",
					"method" : "method",
					"params" : { 
						"param1" : 44, 
						"param2" : { 
							"val_1" : 37,
							"val_2" : "juno",
							"val_3" : "0x4437"
						}
					},
					"id"     : 44
				}`,
		},
		{
			handler: func(param1 int, param2 arg) (any, error) {
				assert.Equal(t, 44, param1)
				assert.Equal(t, arg{
					Val1: 37,
					Val2: "juno",
					Val3: felt,
				}, param2)
				return 0, nil
			},
			req: `{
					"jsonrpc" : "2.0",
					"method" : "method",
					"params" : [ 44, { 
							"val_1" : 37,
							"val_2" : "juno",
							"val_3" : "0x4437"
						}],
					"id"     : 44
				}`,
		},
		{
			handler: func(param1 int, param2 arg) (any, error) {
				assert.Equal(t, true, false) // should never be called
				return 0, nil
			},
			req: `{
					"jsonrpc" : "2.0",
					"method" : "method",
					"params" : { 
						"param2" : { 
							"val_1" : 37,
							"val_2" : "juno",
							"val_3" : "0x4437"
						}
					},
					"id"     : 44
				}`,
			shouldFail: true,
			errorMsg:   "missing non-optional param",
		},
		{
			handler: func(param1 int, param2 arg) (any, error) {
				assert.Equal(t, true, false) // should never be called
				return 0, nil
			},
			req: `{
					"jsonrpc" : "2.0",
					"method" : "method",
					"params" : [ { 
							"val_1" : 37,
							"val_2" : "juno",
							"val_3" : "0x4437"
						}, 44],
					"id"     : 44
				}`,
			shouldFail: true,
			errorMsg:   "json: cannot unmarshal object into Go value of type int",
		},
		{
			handler: func(param1 int, param2 arg) (any, error) {
				assert.Equal(t, true, false) // should never be called
				return 0, nil
			},
			req: `{
					"jsonrpc" : "2.0",
					"method" : "method",
					"params" : [ { 
							"val_1" : 37,
							"val_2" : "juno",
							"val_3" : "0x4437"
						}],
					"id"     : 44
				}`,
			shouldFail: true,
			errorMsg:   "missing param in list",
		},
	}

	for _, test := range tests {
		req, err := newRequest(bytes.NewReader([]byte(test.req)))
		if err != nil {
			t.Error(err)
		}

		args, err := buildArguments(req.Params, test.handler, paramNames)
		if !test.shouldFail && err != nil {
			t.Error(err)
		} else if !test.shouldFail {
			reflect.ValueOf(test.handler).Call(args)
		} else {
			assert.EqualError(t, err, test.errorMsg)
		}
	}
}
