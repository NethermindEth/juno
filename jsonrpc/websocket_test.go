package jsonrpc_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/coder/websocket"
	"github.com/sourcegraph/conc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The caller is responsible for closing the connection.
func testConnection(t *testing.T, ctx context.Context, method jsonrpc.Method, listener jsonrpc.EventListener) *websocket.Conn {
	rpc := jsonrpc.NewServer(1, log.NewNopZapLogger()).WithListener(listener)
	require.NoError(t, rpc.RegisterMethods(method))

	// Server
	srv := httptest.NewServer(jsonrpc.NewWebsocket(rpc, nil, log.NewNopZapLogger()))

	// Client
	conn, resp, err := websocket.Dial(ctx, srv.URL, nil) //nolint:bodyclose // websocket package closes resp.Body for us.
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

func TestWebsocketRequestTimeout(t *testing.T) {
	t.Parallel()

	echo := jsonrpc.Method{
		Name:   "test_echo",
		Params: []jsonrpc.Parameter{{Name: "msg"}},
		Handler: func(msg string) (string, *jsonrpc.Error) {
			return msg, nil
		},
	}
	block := jsonrpc.Method{
		Name: "test_block",
		Handler: func(ctx context.Context) (string, *jsonrpc.Error) {
			<-ctx.Done()
			return "", jsonrpc.Err(jsonrpc.InternalError, ctx.Err().Error())
		},
	}

	rpc := jsonrpc.NewServer(1, log.NewNopZapLogger())
	require.NoError(t, rpc.RegisterMethods(echo, block))
	ws := jsonrpc.NewWebsocket(rpc, nil, log.NewNopZapLogger()).
		WithRequestTimeout(50 * time.Millisecond)
	srv := httptest.NewServer(ws)
	t.Cleanup(srv.Close)

	conn, resp, err := websocket.Dial(t.Context(), srv.URL, nil) //nolint:bodyclose // lib closes Body
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)

	req := `{"jsonrpc" : "2.0", "method" : "test_block", "params":[], "id" : 1}`
	require.NoError(t, conn.Write(t.Context(), websocket.MessageText, []byte(req)))
	_, got, err := conn.Read(t.Context())
	require.NoError(t, err)
	assert.Contains(t, string(got), "context deadline exceeded")

	req = `{"jsonrpc" : "2.0", "method" : "test_echo", "params" : [ "abc123" ], "id" : 2}`
	require.NoError(t, conn.Write(t.Context(), websocket.MessageText, []byte(req)))
	_, got, err = conn.Read(t.Context())
	require.NoError(t, err)
	assert.Equal(t, `{"jsonrpc":"2.0","result":"abc123","id":2}`, string(got))

	require.NoError(t, conn.Close(websocket.StatusNormalClosure, ""))
}

func TestWebsocketBatchRequestSharesDeadline(t *testing.T) {
	t.Parallel()

	echo := jsonrpc.Method{
		Name:   "test_echo",
		Params: []jsonrpc.Parameter{{Name: "msg"}},
		Handler: func(msg string) (string, *jsonrpc.Error) {
			return msg, nil
		},
	}
	block := jsonrpc.Method{
		Name: "test_block",
		Handler: func(ctx context.Context) (string, *jsonrpc.Error) {
			<-ctx.Done()
			return "", jsonrpc.Err(jsonrpc.InternalError, ctx.Err().Error())
		},
	}

	rpc := jsonrpc.NewServer(2, log.NewNopZapLogger())
	require.NoError(t, rpc.RegisterMethods(echo, block))
	ws := jsonrpc.NewWebsocket(rpc, nil, log.NewNopZapLogger()).
		WithRequestTimeout(50 * time.Millisecond)
	srv := httptest.NewServer(ws)
	t.Cleanup(srv.Close)

	conn, resp, err := websocket.Dial(t.Context(), srv.URL, nil) //nolint:bodyclose // lib closes Body
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)

	req := `[{"jsonrpc":"2.0","method":"test_block","params":[],"id":1},` +
		`{"jsonrpc":"2.0","method":"test_echo","params":["abc123"],"id":2}]`
	require.NoError(t, conn.Write(t.Context(), websocket.MessageText, []byte(req)))
	_, got, err := conn.Read(t.Context())
	require.NoError(t, err)

	var batch []json.RawMessage
	require.NoError(t, json.Unmarshal(got, &batch))
	require.Len(t, batch, 2)
	assert.Contains(t, string(got), "context deadline exceeded")
	assert.Contains(t, string(got), `"result":"abc123"`)

	require.NoError(t, conn.Close(websocket.StatusNormalClosure, ""))
}

func TestWebsocketRequestTimeoutDisabled(t *testing.T) {
	t.Parallel()

	hasDeadline := jsonrpc.Method{
		Name: "test_deadline",
		Handler: func(ctx context.Context) (bool, *jsonrpc.Error) {
			_, ok := ctx.Deadline()
			return ok, nil
		},
	}

	rpc := jsonrpc.NewServer(1, log.NewNopZapLogger())
	require.NoError(t, rpc.RegisterMethods(hasDeadline))
	ws := jsonrpc.NewWebsocket(rpc, nil, log.NewNopZapLogger()).WithRequestTimeout(0)
	srv := httptest.NewServer(ws)
	t.Cleanup(srv.Close)

	conn, resp, err := websocket.Dial(t.Context(), srv.URL, nil) //nolint:bodyclose // lib closes Body
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)

	req := `{"jsonrpc" : "2.0", "method" : "test_deadline", "params":[], "id" : 1}`
	require.NoError(t, conn.Write(t.Context(), websocket.MessageText, []byte(req)))
	_, got, err := conn.Read(t.Context())
	require.NoError(t, err)
	assert.Equal(t, `{"jsonrpc":"2.0","result":false,"id":1}`, string(got))

	require.NoError(t, conn.Close(websocket.StatusNormalClosure, ""))
}

func TestWebsocketConnOutlivesRequest(t *testing.T) {
	t.Parallel()

	wg := conc.NewWaitGroup()
	t.Cleanup(wg.Wait)

	method := jsonrpc.Method{
		Name: "test_sub",
		Handler: func(ctx context.Context) (int, *jsonrpc.Error) {
			conn, ok := jsonrpc.ConnFromContext(ctx)
			if !assert.True(t, ok) {
				return 0, jsonrpc.Err(jsonrpc.InternalError, "no conn in context")
			}
			wg.Go(func() {
				<-ctx.Done()
				assert.NoError(t, conn.Context().Err())
				_, werr := conn.Write([]byte("alive"))
				assert.NoError(t, werr)
			})
			return 0, nil
		},
	}

	rpc := jsonrpc.NewServer(1, log.NewNopZapLogger())
	require.NoError(t, rpc.RegisterMethods(method))
	ws := jsonrpc.NewWebsocket(rpc, nil, log.NewNopZapLogger()).
		WithRequestTimeout(50 * time.Millisecond)
	srv := httptest.NewServer(ws)
	t.Cleanup(srv.Close)

	conn, resp, err := websocket.Dial(t.Context(), srv.URL, nil) //nolint:bodyclose // lib closes Body
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)

	req := `{"jsonrpc" : "2.0", "method" : "test_sub", "params":[], "id" : 1}`
	require.NoError(t, conn.Write(t.Context(), websocket.MessageText, []byte(req)))

	_, got, err := conn.Read(t.Context())
	require.NoError(t, err)
	assert.Equal(t, `{"jsonrpc":"2.0","result":0,"id":1}`, string(got))

	_, alive, err := conn.Read(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "alive", string(alive))

	require.NoError(t, conn.Close(websocket.StatusNormalClosure, ""))
}

func TestWebsocketConnectionLimit(t *testing.T) {
	t.Parallel()

	rpc := jsonrpc.NewServer(1, log.NewNopZapLogger())
	ws := jsonrpc.NewWebsocket(rpc, nil, log.NewNopZapLogger()).WithMaxConnections(2)
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

func TestWebsocketGateRejectsWhenBusy(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	block := jsonrpc.Method{
		Name: "test_block",
		Handler: func(ctx context.Context) (int, *jsonrpc.Error) {
			close(started)
			<-release
			return 0, nil
		},
	}
	echo := jsonrpc.Method{
		Name:    "test_echo",
		Params:  []jsonrpc.Parameter{{Name: "msg"}},
		Handler: func(msg string) (string, *jsonrpc.Error) { return msg, nil },
	}

	rpc := jsonrpc.NewServer(1, log.NewNopZapLogger())
	require.NoError(t, rpc.RegisterMethods(block, echo))
	gate := jsonrpc.NewGate(1, 0)
	ws := jsonrpc.NewWebsocket(rpc, nil, log.NewNopZapLogger()).WithGate(gate)
	srv := httptest.NewServer(ws)
	t.Cleanup(srv.Close)

	connA, respA, err := websocket.Dial(t.Context(), srv.URL, nil) //nolint:bodyclose // lib closes it
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, respA.StatusCode)
	defer connA.Close(websocket.StatusNormalClosure, "")
	require.NoError(t, connA.Write(t.Context(), websocket.MessageText,
		[]byte(`{"jsonrpc":"2.0","method":"test_block","params":[],"id":1}`)))
	<-started

	connB, respB, err := websocket.Dial(t.Context(), srv.URL, nil) //nolint:bodyclose // lib closes it
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, respB.StatusCode)
	defer connB.Close(websocket.StatusNormalClosure, "")
	require.NoError(t, connB.Write(t.Context(), websocket.MessageText,
		[]byte(`{"jsonrpc":"2.0","method":"test_echo","params":["hi"],"id":2}`)))
	_, got, err := connB.Read(t.Context())
	require.NoError(t, err)
	assert.Equal(t,
		`{"jsonrpc":"2.0","error":{"code":-32603,"message":"server busy"},"id":null}`,
		string(got))

	close(release)
	_, _, err = connA.Read(t.Context())
	require.NoError(t, err)
	require.Eventually(t, func() bool { return gate.Running() == 0 }, time.Second, 5*time.Millisecond)

	require.NoError(t, connB.Write(t.Context(), websocket.MessageText,
		[]byte(`{"jsonrpc":"2.0","method":"test_echo","params":["hi"],"id":3}`)))
	_, got, err = connB.Read(t.Context())
	require.NoError(t, err)
	assert.Equal(t, `{"jsonrpc":"2.0","result":"hi","id":3}`, string(got))
}
