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
func testConnection(t *testing.T, ctx context.Context, method jsonrpc.Method, listener jsonrpc.EventListener) *websocket.Conn {
	rpc := jsonrpc.NewServer(1, utils.NewNopZapLogger()).WithListener(listener)
	require.NoError(t, rpc.RegisterMethods(method))

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

	method := jsonrpc.Method{
		Name:   "test_echo",
		Params: []jsonrpc.Parameter{{Name: "msg"}},
		Handler: func(msg string) (string, *jsonrpc.Error) {
			return msg, nil
		},
	}
	listener := CountingEventListener{}
	conn := testConnection(t, ctx, method, &listener)

	msg := `{"jsonrpc" : "2.0", "method" : "test_echo", "params" : [ "abc123" ], "id" : 1}`
	err := conn.Write(context.Background(), websocket.MessageText, []byte(msg))
	require.NoError(t, err)

	want := `{"jsonrpc":"2.0","result":"abc123","id":1}`
	_, got, err := conn.Read(context.Background())
	require.NoError(t, err)
	assert.Equal(t, want, string(got))
	assert.Len(t, listener.OnNewRequestLogs, 1)

	require.NoError(t, conn.Close(websocket.StatusNormalClosure, ""))
}

func TestSendFromHandler(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msg := "test msg"
	method := jsonrpc.Method{
		Name: "test",
		Handler: func(ctx context.Context) (int, *jsonrpc.Error) {
			conn, ok := jsonrpc.ConnFromContext(ctx)
			require.True(t, ok)
			_, err := conn.Write([]byte(msg))
			require.NoError(t, err)
			return 0, nil
		},
	}
	conn := testConnection(t, ctx, method, &CountingEventListener{})

	req := `{"jsonrpc" : "2.0", "method" : "test", "params":[], "id" : 1}`
	err := conn.Write(context.Background(), websocket.MessageText, []byte(req))
	require.NoError(t, err)

	_, resp1, err := conn.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, msg, string(resp1))

	want := `{"jsonrpc":"2.0","result":0,"id":1}`
	_, got, err := conn.Read(ctx)
	require.NoError(t, err)
	assert.Equal(t, want, string(got))

	require.NoError(t, conn.Close(websocket.StatusNormalClosure, ""))
}
