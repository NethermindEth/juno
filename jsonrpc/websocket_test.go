package jsonrpc_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/utils"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConnection(t *testing.T, port uint16) *websocket.Conn {
	methods := []jsonrpc.Method{{
		Name:   "test_echo",
		Params: []jsonrpc.Parameter{{Name: "msg"}},
		Handler: func(msg string) (string, *jsonrpc.Error) {
			return msg, nil
		},
	}}
	ws := jsonrpc.NewWebsocket(port, methods, utils.NewNopZapLogger())
	ws.WithConnectionParams(jsonrpc.DefaultWebsocketConnectionParams())
	go func() {
		require.NoError(t, ws.Run(context.Background()))
	}()

	url := fmt.Sprintf("ws://localhost:%d", port)
	conn, resp, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
	require.NoError(t, resp.Body.Close())
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	return conn
}

func TestHandler(t *testing.T) {
	conn := testConnection(t, 8457)

	msg := `{"jsonrpc" : "2.0", "method" : "test_echo", "params" : [ "abc123" ], "id" : 1}`
	err := conn.WriteMessage(websocket.TextMessage, []byte(msg))
	require.NoError(t, err)

	want := `{"jsonrpc":"2.0","result":"abc123","id":1}`
	_, got, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, want, string(got))
}

func TestPongRespondsToPing(t *testing.T) {
	conn := testConnection(t, 8458)

	pongReceived := false
	conn.SetPongHandler(func(_ string) error {
		pongReceived = true
		return nil
	})

	require.NoError(t, conn.WriteMessage(websocket.PingMessage, nil))
	// Send a text message to unblock the call to NextReader() below.
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, nil))
	_, _, err := conn.NextReader()
	require.NoError(t, err)

	assert.True(t, pongReceived)
}
