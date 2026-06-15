package client_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/NethermindEth/juno/l1/eth/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRPCError_Error(t *testing.T) {
	cases := []struct {
		name string
		err  client.RPCError
		want string
	}{
		{
			name: "without data",
			err:  client.RPCError{Code: -32601, Message: "the method does not exist"},
			want: "json-rpc error -32601: the method does not exist",
		},
		{
			name: "with data",
			err: client.RPCError{
				Code:    -32000,
				Message: "execution reverted",
				Data:    json.RawMessage(`"0xabcd"`),
			},
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
	result, err := client.DecodeResponse(body, 7)
	require.NoError(t, err)
	assert.JSONEq(t, `"0x539"`, string(result))
}

func TestDecodeResponse_NullResult(t *testing.T) {
	// A successful response with a null result (e.g. missing receipt or
	// missing header) is NOT an error at the wire layer; the methods
	// layer interprets null per-method.
	body := []byte(`{"jsonrpc":"2.0","id":3,"result":null}`)
	result, err := client.DecodeResponse(body, 3)
	require.NoError(t, err)
	assert.JSONEq(t, `null`, string(result))
}

func TestDecodeResponse_ServerError(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","id":11,"error":{"code":-32000,"message":"header not found"}}`)
	_, err := client.DecodeResponse(body, 11)
	require.Error(t, err)

	var rerr *client.RPCError
	require.True(t, errors.As(err, &rerr), "expected *client.RPCError, got %T", err)
	assert.Equal(t, -32000, rerr.Code)
	assert.Equal(t, "header not found", rerr.Message)
}

func TestDecodeResponse_IDMismatch(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","id":99,"result":"0x1"}`)
	_, err := client.DecodeResponse(body, 1)
	require.Error(t, err)
	require.ErrorIs(t, err, client.ErrIDMismatch)
}

// TestDecodeResponse_StringID covers spec-tolerance for servers that
// echo our outgoing uint64 as a JSON string. Matches juno's own
// jsonrpc/server.go which accepts both shapes.
func TestDecodeResponse_StringID(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","id":"7","result":"0x539"}`)
	result, err := client.DecodeResponse(body, 7)
	require.NoError(t, err)
	assert.JSONEq(t, `"0x539"`, string(result))
}

// TestDecodeResponse_NullID rejects null ids — a null id in a response
// signals a server-side parse failure, not "my reply matches your
// request". Treat it as an id mismatch so callers don't accidentally
// consume the result.
func TestDecodeResponse_NullID(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","id":null,"result":"0x1"}`)
	_, err := client.DecodeResponse(body, 1)
	require.Error(t, err)
	require.ErrorIs(t, err, client.ErrIDMismatch)
}

// TestDecodeResponse_NonNumericStringID rejects non-numeric string ids
// since our outgoing ids are always uint64-shaped — anything else can't
// possibly match.
func TestDecodeResponse_NonNumericStringID(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","id":"abc","result":"0x1"}`)
	_, err := client.DecodeResponse(body, 1)
	require.Error(t, err)
	require.ErrorIs(t, err, client.ErrIDMismatch)
}

func TestDecodeResponse_MalformedJSON(t *testing.T) {
	_, err := client.DecodeResponse([]byte(`{"jsonrpc":"2.0","id":1`), 1)
	require.Error(t, err)
	// Wrapped json error — assert by message fragment to stay decoupled
	// from the stdlib's internal phrasing.
	assert.Contains(t, err.Error(), "decode response")
}

func TestDecodeResponse_PreservesErrorData(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","id":1,` +
		`"error":{"code":-32000,"message":"execution reverted","data":"0xdeadbeef"}}`)
	_, err := client.DecodeResponse(body, 1)
	require.Error(t, err)
	var rerr *client.RPCError
	require.True(t, errors.As(err, &rerr))
	assert.JSONEq(t, `"0xdeadbeef"`, string(rerr.Data))
}
