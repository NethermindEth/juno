package jsonrpc

import (
	"encoding/json"
	"errors"
	"testing"

	"gotest.tools/v3/assert"
)

type customError struct{}

func (*customError) Error() string {
	return "custom error"
}

func (c *customError) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.Error())
}

type Service struct{}

func (*Service) Sum(a, b int) (any, error) {
	return a + b, nil
}

func (*Service) InvalidOut1() {}

func (*Service) InvalidOut2() (error, error) {
	return nil, nil
}

func (*Service) InvalidOut3() (any, any) {
	return nil, nil
}

func (*Service) ReturnError() (any, error) {
	return nil, &customError{}
}

func TestServer_RegisterFunc(t *testing.T) {
	server := NewJsonRpc()
	service := &Service{}
	type args struct {
		name       string
		paramNames []string
		function   any
	}
	tests := []struct {
		name string
		args args
		want error
	}{
		{
			name: "invalid function",
			args: args{
				name:       "service_invalidFunc",
				paramNames: []string{},
				function:   &Service{},
			},
			want: ErrInvalidFunc,
		},
		{
			name: "invalid function",
			args: args{
				name:       "service_sum",
				paramNames: []string{"a"},
				function:   service.Sum,
			},
			want: ErrInvalidFunc,
		},
		{
			name: "correct register",
			args: args{
				name:       "service_sum",
				paramNames: []string{"a", "b"},
				function:   service.Sum,
			},
			want: nil,
		},
		{
			name: "already exists",
			args: args{
				name:       "service_sum",
				paramNames: []string{"a", "b"},
				function:   service.Sum,
			},
			want: ErrAlreadyExists,
		},
		{
			name: "invalid output",
			args: args{
				name:       "service_invalidOut1",
				paramNames: []string{},
				function:   service.InvalidOut1,
			},
			want: ErrInvalidFunc,
		},
		{
			name: "invalid output",
			args: args{
				name:       "service_invalidOut2",
				paramNames: []string{},
				function:   service.InvalidOut2,
			},
			want: ErrInvalidFunc,
		},
		{
			name: "invalid output",
			args: args{
				name:       "service_invalidOut3",
				paramNames: []string{},
				function:   service.InvalidOut3,
			},
			want: ErrInvalidFunc,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := server.RegisterFunc(tt.args.name, tt.args.function, tt.args.paramNames...)
			if !errors.Is(out, tt.want) {
				t.Errorf("RegisterFunc() = %v, want %v", out, tt.want)
			}
		})
	}
}

func TestServer_Call(t *testing.T) {
	server := NewJsonRpc()
	service := &Service{}
	if err := server.RegisterFunc("service_sum", service.Sum, "a", "b"); err != nil {
		t.Fatal(err)
	}
	if err := server.RegisterFunc("service_returnError", service.ReturnError); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		args []byte
		want []byte
	}{
		{
			name: "sum (param list)",
			args: []byte(`{"jsonrpc":"2.0","method":"service_sum","params":[1,2],"id":1}`),
			want: []byte(`{"jsonrpc":"2.0","result":3,"id":1}`),
		},
		{
			name: "sum (named params)",
			args: []byte(`{"jsonrpc":"2.0","method":"service_sum","params":{"a":1,"b":2},"id":1}`),
			want: []byte(`{"jsonrpc":"2.0","result":3,"id":1}`),
		},
		{
			name: "sum (default values, param list)",
			args: []byte(`{"jsonrpc":"2.0","method":"service_sum","params":[],"id":1}`),
			want: []byte(`{"jsonrpc":"2.0","result":0,"id":1}`),
		},
		{
			name: "sum (default values, named params)",
			args: []byte(`{"jsonrpc":"2.0","method":"service_sum","params":{},"id":"1"}`),
			want: []byte(`{"jsonrpc":"2.0","result":0,"id":"1"}`),
		},
		{
			name: "call not found method",
			args: []byte(`{"jsonrpc":"2.0","method":"service_notFound","params":[],"id":1}`),
			want: []byte(`{"jsonrpc":"2.0","error":{"code":-32601,"message":"Method not found"},"id":1}`),
		},
		{
			name: "invalid param type",
			args: []byte(`{"jsonrpc":"2.0","method":"service_sum","params":1,"id":1}`),
			want: []byte(`{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid params"},"id":1}`),
		},
		{
			name: "invalid list param type",
			args: []byte(`{"jsonrpc":"2.0","method":"service_sum","params":[1,"2"],"id":1}`),
			want: []byte(`{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid params"},"id":1}`),
		},
		{
			name: "invalid named param type",
			args: []byte(`{"jsonrpc":"2.0","method":"service_sum","params":{"a":1,"b":"2"},"id":1}`),
			want: []byte(`{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid params"},"id":1}`),
		},
		{
			name: "invalid list param length",
			args: []byte(`{"jsonrpc":"2.0","method":"service_sum","params":[1,2,3],"id":1}`),
			want: []byte(`{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid params"},"id":1}`),
		},
		{
			name: "return error",
			args: []byte(`{"jsonrpc":"2.0","method":"service_returnError","params":[],"id":1}`),
			want: []byte(`{"jsonrpc":"2.0","error":"custom error","id":1}`),
		},
		{
			name: "invalid jsonrpc version",
			args: []byte(`{"jsonrpc":"2.1","method":"service_sum","params":[1,2],"id":"1"}`),
			want: []byte(`{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request"}}`),
		},
		{
			name: "invalid jsonrpc version type",
			args: []byte(`{"jsonrpc":{},"method":"service_sum","params":[1,2],"id":"1"}`),
			want: []byte(`{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request"}}`),
		},
		{
			name: "invalid request id type (float)",
			args: []byte(`{"jsonrpc":"2.0","method":"service_sum","params":[1,2],"id":1.0}`),
			want: []byte(`{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request"}}`),
		},
		{
			name: "invalid request id type (bool)",
			args: []byte(`{"jsonrpc":"2.0","method":"service_sum","params":[1,2],"id":true}`),
			want: []byte(`{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request"}}`),
		},
		{
			name: "invalid params object",
			args: []byte(`{"jsonrpc":"2.0","method":"service_sum","params":[1,],"id":"1"}`),
			want: []byte(`{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error"}}`),
		},
		{
			name: "notification",
			args: []byte(`{"jsonrpc":"2.0","method":"service_sum","params":[1,2]}`),
			want: nil,
		},
		{
			name: "batch",
			args: []byte(`[{"jsonrpc":"2.0","method":"service_sum","params":[1,2],"id":1},{"jsonrpc":"2.0","method":"service_sum","params":[1,2],"id":1}]`),
			want: []byte(`[{"jsonrpc":"2.0","result":3,"id":1},{"jsonrpc":"2.0","result":3,"id":1}]`),
		},
		{
			name: "empty batch",
			args: []byte(`[]`),
			want: []byte(`{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request"}}`),
		},
		{
			name: "invalid batch",
			args: []byte(`[`),
			want: []byte(`{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error"}}`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := server.Call(tt.args)
			if tt.want != nil {
				assert.DeepEqual(t, string(tt.want), string(out))
			}
		})
	}
}
