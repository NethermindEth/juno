package jsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

type TestService struct{}

type SubstractArgs struct {
	Minuend     int `json:"minuend"`
	Substrahend int `json:"substrahend"`
}

func (s *TestService) Subtract(ctx context.Context, args *SubstractArgs) (any, error) {
	return args.Minuend - args.Substrahend, nil
}

type UpdateArgs struct {
	A, B, C, D, E int
}

func (s *TestService) Update(ctx context.Context, args *UpdateArgs) (any, error) {
	return nil, nil
}

func (s *TestService) Foobar(ctx context.Context) (any, error) {
	return nil, nil
}

type SumArgs struct {
	A, B, C int
}

func (s *TestService) Sum(ctx context.Context, args *SumArgs) (any, error) {
	return args.A + args.B + args.C, nil
}

type NotifyService struct{}

type HelloArgs struct {
	Value int
}

func (s *NotifyService) Hello(ctx context.Context, args *HelloArgs) (any, error) {
	return nil, nil
}

func TestServer_Call(t *testing.T) {
	tests := []struct {
		name string
		req  json.RawMessage
		want json.RawMessage
	}{
		{
			name: "with positional arguments",
			req:  []byte(`{"jsonrpc":"2.0","method":"subtract","params":[42,23],"id":1}`),
			want: []byte(`{"jsonrpc":"2.0","result":19,"id":1}`),
		},
		{
			name: "with positional arguments",
			req:  []byte(`{"jsonrpc":"2.0","method":"subtract","params":[23,42],"id":1}`),
			want: []byte(`{"jsonrpc":"2.0","result":-19,"id":1}`),
		},
		{
			name: "with named arguments",
			req:  []byte(`{"jsonrpc":"2.0","method":"subtract","params":{"substrahend":23,"minuend":42},"id":3}`),
			want: []byte(`{"jsonrpc":"2.0","result":19,"id":3}`),
		},
		{
			name: "with named arguments",
			req:  []byte(`{"jsonrpc":"2.0","method":"subtract","params":{"minuend":42,"substrahend":23},"id":4}`),
			want: []byte(`{"jsonrpc":"2.0","result":19,"id":4}`),
		},
		{
			name: "a notification",
			req:  []byte(`{"jsonrpc":"2.0","method":"update","params":[1,2,3,4,5]}`),
			want: nil,
		},
		{
			name: "a notification without params",
			req:  []byte(`{"jsonrpc":"2.0","method":"foobar"}`),
			want: nil,
		},
		{
			name: "call non-existent method",
			req:  []byte(`{"jsonrpc":"2.0","method":"foobar123","id":1}`),
			want: []byte(`{"jsonrpc":"2.0","error":{"code":-32601,"message":"Method not found"},"id":1}`),
		},
		{
			name: "call with invalid json",
			req:  []byte(`{"jsonrpc": "2.0", "method": "foobar, "params": "bar", "baz]`),
			want: []byte(`{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error"},"id":null}`),
		},
		{
			name: "call with invalid request object",
			req:  []byte(`{"jsonrpc": "2.0", "method": 1, "params": "bar"}`),
			want: []byte(`{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request"},"id":null}`),
		},
		{
			name: "call batch invalid json",
			req:  []byte(`[{"jsonrpc": "2.0", "method": "sum", "params": [1,2,4], "id": "1"},{"jsonrpc": "2.0", "method"]`),
			want: []byte(`{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error"},"id":null}`),
		},
		{
			name: "call with empty array",
			req:  []byte(`[]`),
			want: []byte(`{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request"},"id":null}`),
		},
		{
			name: "call with an invalid batch",
			req:  []byte(`[1]`),
			want: []byte(`[
				{"jsonrpc": "2.0", "error": {"code": -32600, "message": "Invalid Request"}, "id": null}
			]`),
		},
		{
			name: "call with an invalid batch",
			req:  []byte(`[1,2,3]`),
			want: []byte(`[
                {"jsonrpc": "2.0", "error": {"code": -32600, "message": "Invalid Request"}, "id": null},
                {"jsonrpc": "2.0", "error": {"code": -32600, "message": "Invalid Request"}, "id": null},
                {"jsonrpc": "2.0", "error": {"code": -32600, "message": "Invalid Request"}, "id": null}
            ]`),
		},
		{
			name: "call batch",
			req: []byte(`[
                {"jsonrpc": "2.0", "method": "sum", "params": [1,2,4], "id": "1"},
				{"jsonrpc": "2.0", "method": "notify_hello", "params": [7]}
            ]`),
			want: []byte(`[
        		{"jsonrpc": "2.0", "result": 7, "id": "1"}
    		]`),
		},
		{
			name: "invalid request",
			req:  []byte(`{"foo": "boo"}`),
			want: []byte(`{"jsonrpc": "2.0", "error": {"code": -32600, "message": "Invalid Request"}, "id": null}`),
		},
	}
	server := NewServer()
	if err := server.RegisterService("", &TestService{}); err != nil {
		t.Error(err)
	}
	if err := server.RegisterService("notify", &NotifyService{}); err != nil {
		t.Error(err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := server.Call(tt.req)
			if tt.want != nil {
				require.JSONEq(t, string(tt.want), string(got))
			} else {
				fmt.Println(string(got))
				require.Nil(t, got)
			}
		})
	}
}
