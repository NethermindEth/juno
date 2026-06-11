package client

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRPCError_Error(t *testing.T) {
	cases := []struct {
		name string
		err  rpcError
		want string
	}{
		{
			name: "without data",
			err:  rpcError{Code: -32601, Message: "the method does not exist"},
			want: "json-rpc error -32601: the method does not exist",
		},
		{
			name: "with data",
			err:  rpcError{Code: -32000, Message: "execution reverted", Data: json.RawMessage(`"0xabcd"`)},
			want: `json-rpc error -32000: execution reverted: "0xabcd"`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, c.err.Error())
		})
	}
}

func TestDecodeResponse_Success(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","id":7,"result":"0x539"}`)
	result, err := decodeResponse(body, 7)
	require.NoError(t, err)
	assert.JSONEq(t, `"0x539"`, string(result))
}

func TestDecodeResponse_NullResult(t *testing.T) {
	// A successful response with a null result (e.g. missing receipt or
	// missing header) is NOT an error at the wire layer; the methods
	// layer interprets null per-method.
	body := []byte(`{"jsonrpc":"2.0","id":3,"result":null}`)
	result, err := decodeResponse(body, 3)
	require.NoError(t, err)
	assert.JSONEq(t, `null`, string(result))
}

func TestDecodeResponse_ServerError(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","id":11,"error":{"code":-32000,"message":"header not found"}}`)
	_, err := decodeResponse(body, 11)
	require.Error(t, err)

	var rerr *rpcError
	require.True(t, errors.As(err, &rerr), "expected *rpcError, got %T", err)
	assert.Equal(t, -32000, rerr.Code)
	assert.Equal(t, "header not found", rerr.Message)
}

func TestDecodeResponse_IDMismatch(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","id":99,"result":"0x1"}`)
	_, err := decodeResponse(body, 1)
	require.Error(t, err)
	require.ErrorIs(t, err, errIDMismatch)
}

func TestDecodeResponse_MalformedJSON(t *testing.T) {
	_, err := decodeResponse([]byte(`{"jsonrpc":"2.0","id":1`), 1)
	require.Error(t, err)
	// Wrapped json error — assert by message fragment to stay decoupled
	// from the stdlib's internal phrasing.
	assert.Contains(t, err.Error(), "decode response")
}

func TestDecodeResponse_PreservesErrorData(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"execution reverted","data":"0xdeadbeef"}}`)
	_, err := decodeResponse(body, 1)
	require.Error(t, err)
	var rerr *rpcError
	require.True(t, errors.As(err, &rerr))
	assert.JSONEq(t, `"0xdeadbeef"`, string(rerr.Data))
}
