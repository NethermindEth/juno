package jsonrpc_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"nhooyr.io/websocket"
)

// The caller is responsible for closing the connection.
func testConnection(t *testing.T, ctx context.Context) *websocket.Conn {
	method := jsonrpc.Method{
		Name:   "test_echo",
		Params: []jsonrpc.Parameter{{Name: "msg"}},
		Handler: func(msg string) (string, *jsonrpc.Error) {
			return msg, nil
		},
	}
	rpc := jsonrpc.NewServer(utils.NewNopZapLogger())
	require.NoError(t, rpc.RegisterMethod(method))

	// Server
	srv := httptest.NewServer(jsonrpc.NewWebsocket(rpc, utils.NewNopZapLogger()))

	// Client
	conn, resp, err := websocket.Dial(ctx, srv.URL, nil) //nolint:bodyclose // websocket package closes resp.Body for us.
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)

	return conn
}

func TestHandler(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conn := testConnection(t, ctx)

	msg := `{"jsonrpc" : "2.0", "method" : "test_echo", "params" : [ "abc123" ], "id" : 1}`
	err := conn.Write(context.Background(), websocket.MessageText, []byte(msg))
	require.NoError(t, err)

	want := `{"jsonrpc":"2.0","result":"abc123","id":1}`
	_, got, err := conn.Read(context.Background())
	require.NoError(t, err)
	assert.Equal(t, want, string(got))

	require.NoError(t, conn.Close(websocket.StatusNormalClosure, ""))
}
