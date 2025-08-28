package jsonrpc_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/utils"
	"github.com/coder/websocket"
	"github.com/sourcegraph/conc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The caller is responsible for closing the connection.
func testConnection(
	t *testing.T,
	ctx context.Context,
	method jsonrpc.Method,
	listener jsonrpc.EventListener,
) *websocket.Conn {
	rpc := jsonrpc.NewServer(1, utils.NewNopZapLogger()).WithListener(listener)
	require.NoError(t, rpc.RegisterMethods(method))

	// Server
	srv := httptest.NewServer(jsonrpc.NewWebsocket(rpc, nil, utils.NewNopZapLogger()))

	// Client
	//nolint:bodyclose // websocket package closes resp.Body for us.
	conn, resp, err := websocket.Dial(
		ctx,
		srv.URL,
		nil,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)

	return conn
}

func TestHandler(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
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
	err := conn.Write(t.Context(), websocket.MessageText, []byte(msg))
	require.NoError(t, err)

	want := `{"jsonrpc":"2.0","result":"abc123","id":1}`
	_, got, err := conn.Read(t.Context())
	require.NoError(t, err)
	assert.Equal(t, want, string(got))
	assert.Len(t, listener.OnNewRequestLogs, 1)

	require.NoError(t, conn.Close(websocket.StatusNormalClosure, ""))
}

func TestSendFromHandler(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	wg := conc.NewWaitGroup()
	t.Cleanup(wg.Wait)
	msg := "test msg"
	method := jsonrpc.Method{
		Name: "test",
		Handler: func(ctx context.Context) (int, *jsonrpc.Error) {
			conn, ok := jsonrpc.ConnFromContext(ctx)
			require.True(t, ok)
			wg.Go(func() {
				_, err := conn.Write([]byte(msg))
				require.NoError(t, err)
			})
			return 0, nil
		},
	}
	conn := testConnection(t, ctx, method, &CountingEventListener{})

	req := `{"jsonrpc" : "2.0", "method" : "test", "params":[], "id" : 1}`
	err := conn.Write(t.Context(), websocket.MessageText, []byte(req))
	require.NoError(t, err)

	want := `{"jsonrpc":"2.0","result":0,"id":1}`
	_, got, err := conn.Read(ctx)
	require.NoError(t, err)
	assert.Equal(t, want, string(got))

	_, resp1, err := conn.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, msg, string(resp1))

	require.NoError(t, conn.Close(websocket.StatusNormalClosure, ""))
}

func TestWebsocketConnectionLimit(t *testing.T) {
	t.Parallel()

	rpc := jsonrpc.NewServer(1, utils.NewNopZapLogger())
	ws := jsonrpc.NewWebsocket(rpc, nil, utils.NewNopZapLogger()).WithMaxConnections(2)
	httpSrv := httptest.NewServer(ws)
	defer httpSrv.Close()

	// First connection should succeed
	conn1, resp1, err := websocket.Dial(t.Context(), httpSrv.URL, nil) //nolint:bodyclose
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, resp1.StatusCode)
	defer conn1.Close(websocket.StatusNormalClosure, "")

	// Second connection should succeed
	conn2, resp2, err := websocket.Dial(t.Context(), httpSrv.URL, nil) //nolint:bodyclose
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, resp2.StatusCode)
	defer conn2.Close(websocket.StatusNormalClosure, "")

	// Third connection should fail with 503 Service Unavailable
	_, resp3, err := websocket.Dial(t.Context(), httpSrv.URL, nil) //nolint:bodyclose
	require.Error(t, err)
	require.Equal(t, http.StatusServiceUnavailable, resp3.StatusCode)

	// Close one connection and try again - should succeed
	require.NoError(t, conn1.Close(websocket.StatusNormalClosure, ""))
	time.Sleep(10 * time.Millisecond) // Give the server time to clean up

	conn4, resp4, err := websocket.Dial(t.Context(), httpSrv.URL, nil) //nolint:bodyclose
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, resp4.StatusCode)
	require.NoError(t, conn4.Close(websocket.StatusNormalClosure, ""))
}
