package jsonrpc

import "testing"

func TestRpcRequest_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		args []byte
		err  error
	}{
		{
			name: "invalid json (extra comma)",
			args: []byte(`
				{
					"jsonrpc": "2.0",
					"method": "service_method1",
					"parmas": [1,2,3,4],
					"id": "1234",
				}
			`),
			err: errParseError,
		},
		{
			name: "invalid json (no object)",
			args: []byte(`1`),
			err:  errInvalidRequest,
		},
		{
			name: "notification",
			args: []byte(`
				{
					"jsonrpc": "2.0",
					"method": "service_method1",
					"params": {
						"param1": "value1",
						"param2": "value2"
					}
				}
			`),
			err: nil,
		},
		{
			name: "number id",
			args: []byte(`
				{
					"jsonrpc": "2.0",
					"method": "service_method1",
					"params": {
						"param1": "value1",
						"param2": "value2"
					},
					"id": 1234
				}
			`),
			err: nil,
		},
		{
			name: "float id",
			args: []byte(`
				{
					"jsonrpc": "2.0",
					"method": "service_method1",
					"params": {
						"param1": "value1",
						"param2": "value2"
					},
					"id": 1234.5
				}
			`),
			err: errInvalidRequest,
		},
		{
			name: "invalid id type",
			args: []byte(`
				{
					"jsonrpc": "2.0",
					"method": "service_method1",
					"params": {
						"param1": "value1",
						"param2": "value2"
					},
					"id": true 
				}
			`),
			err: errInvalidRequest,
		},
		{
			name: "object params",
			args: []byte(`
				{
					"jsonrpc": "2.0",
					"method": "service_method1",
					"params": {
						"param1": "value1",
						"param2": "value2"
					},
					"id": "1234"
				}
			`),
			err: nil,
		},
		{
			name: "object params with multiple types",
			args: []byte(`
				{
					"jsonrpc": "2.0",
					"method": "service_method1",
					"params": {
						"param1": "value1",
						"param2": 3456,
						"param3": true,
						"param4": false,
						"param5": {
							"field1": "value1",
							"field2": 3456,
							"field3": true,
							"field4": false
						}
					},
					"id": "1234"
				}
			`),
			err: nil,
		},
		{
			name: "list params",
			args: []byte(`
				{
					"jsonrpc": "2.0",
					"method": "service_method1",
					"params": ["value1","value2"],
					"id": "1234"
				}
			`),
			err: nil,
		},
		{
			name: "list params with multiple types",
			args: []byte(`
				{
					"jsonrpc": "2.0",
					"method": "service_method1",
					"params": [
						"value1",
						3456,
						true,
						false,
						{
							"field1": "value1",
							"field2": 3456,
							"field3": true,
							"field4": false
						}
					],
					"id": "1234"
				}
			`),
			err: nil,
		},
		{
			name: "without jsonrpc field",
			args: []byte(`
				{
					"method": "service_method1",
					"params": [1,2,3],
					"id": "1234"
				}
			`),
			err: errInvalidRequest,
		},
		{
			name: "with invalid jsonrpc field",
			args: []byte(`
				{
					"jsonrpc": "2.1",
					"method": "service_method1",
					"params": [1,2,3],
					"id": "1234"
				}
			`),
			err: errInvalidRequest,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rpcRequest rpcRequest
			if err := rpcRequest.UnmarshalJSON(tt.args); err != tt.err {
				t.Errorf("RpcRequest.UnmarshalJSON() error = %v, wantErr %v", err, tt.err)
			}
		})
	}
}
