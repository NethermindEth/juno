package jsonrpc_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"nhooyr.io/websocket"
)

const id = 1

var values = []int{1, 2, 3}

// The caller is responsible for closing the connection.
func testConnection(t *testing.T, ctx context.Context) *websocket.Conn {
	method := jsonrpc.Method{
		Name:   "test_echo",
		Params: []jsonrpc.Parameter{{Name: "msg"}},
		Handler: func(msg string) (string, *jsonrpc.Error) {
			return msg, nil
		},
	}
	submethod := jsonrpc.SubscribeMethod{
		Name: "test_subscribe",
		Handler: func(subServer *jsonrpc.SubscriptionServer) (event.Subscription, *jsonrpc.Error) {
			return event.NewSubscription(func(quit <-chan struct{}) error {
				for _, value := range values {
					select {
					case <-quit:
						return nil
					default:
						if err := subServer.Send(value); err != nil {
							return err
						}
					}
				}
				<-quit
				return nil
			}), nil
		},
		UnsubMethodName: "test_unsubscribe",
	}
	rpc := jsonrpc.NewServer(1, utils.NewNopZapLogger())
	rpc.WithIDGen(func() (uint64, error) {
		return id, nil
	})
	require.NoError(t, rpc.RegisterMethod(method))
	rpc.RegisterSubscribeMethod(submethod)

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

func TestWebsocketSubscribeUnsubscribe(t *testing.T) {
	conn := testConnection(t, context.Background())

	// Initial subscription handshake.
	req := `{"jsonrpc": "2.0", "method": "test_subscribe", "params": [], "id": 1}`
	wswrite(t, conn, req)
	want := fmt.Sprintf(`{"jsonrpc":"2.0","result":%d,"id":1}`, id)
	got := wsread(t, conn)
	require.Equal(t, want, got)

	// All values are received.
	for _, value := range values {
		want = fmt.Sprintf(`{"jsonrpc":"2.0","method":"test_subscribe","params":{"subscription":%d,"result":%d}}`, id, value)
		got = wsread(t, conn)
		assert.Equal(t, want, got)
	}

	// Unsubscribe.
	wswrite(t, conn, fmt.Sprintf(unsubscribeTemplate, id))
	want = fmt.Sprintf(`{"jsonrpc":"2.0","result":%d,"id":1}`, id)
	got = wsread(t, conn)
	assert.Equal(t, want, got)

	require.NoError(t, conn.Close(websocket.StatusNormalClosure, ""))
}

func wswrite(t *testing.T, conn *websocket.Conn, req string) {
	err := conn.Write(context.Background(), websocket.MessageText, []byte(req))
	require.NoError(t, err)
}

func wsread(t *testing.T, conn *websocket.Conn) string {
	_, res, err := conn.Read(context.Background())
	require.NoError(t, err)
	return string(res)
}
