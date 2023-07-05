package jsonrpc_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"testing"

	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"nhooyr.io/websocket"
)

// The caller is responsible for closing the connection.
func testConnection(t *testing.T) *websocket.Conn {
	methods := []jsonrpc.Method{{
		Name:   "test_echo",
		Params: []jsonrpc.Parameter{{Name: "msg"}},
		Handler: func(msg string) (string, *jsonrpc.Error) {
			return msg, nil
		},
	}}
	l, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	ctx := context.Background()
	ws := jsonrpc.NewWebsocket(l, methods, utils.NewNopZapLogger(), nil)
	go func() {
		t.Helper()
		require.NoError(t, ws.Run(context.Background()))
	}()

	remote := url.URL{
		Scheme: "ws",
		Host:   fmt.Sprintf("localhost:%d", l.Addr().(*net.TCPAddr).Port),
	}
	conn, resp, err := websocket.Dial(ctx, remote.String(), nil) //nolint:bodyclose // websocket package closes resp.Body for us.
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)

	return conn
}

func TestHandler(t *testing.T) {
	conn := testConnection(t)

	msg := `{"jsonrpc" : "2.0", "method" : "test_echo", "params" : [ "abc123" ], "id" : 1}`
	err := conn.Write(context.Background(), websocket.MessageText, []byte(msg))
	require.NoError(t, err)

	want := `{"jsonrpc":"2.0","result":"abc123","id":1}`
	_, got, err := conn.Read(context.Background())
	require.NoError(t, err)
	assert.Equal(t, want, string(got))

	require.NoError(t, conn.Close(websocket.StatusNormalClosure, ""))
}
