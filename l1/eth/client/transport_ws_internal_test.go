// White-box tests. These two assertions cannot be made through the public API:
// a dead conn triggers both the write path and readLoop's shutdown, so a
// black-box test can't pin WHICH path classified the error, and pendingSubs
// retention is only observable by inspecting the map itself.
package client

import (
	"context"
	"testing"

	"github.com/NethermindEth/juno/l1/internal/clienttest"
	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"
)

func TestWS_WriteFailureClassifiedAsTransportClosed(t *testing.T) {
	srv := clienttest.NewTestServer(t)
	conn, resp, err := websocket.Dial(t.Context(), srv.WSURL(), nil)
	require.NoError(t, err)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	require.NoError(t, conn.CloseNow())

	tr := &wsTransport{
		conn:    conn,
		pending: make(map[uint64]chan rpcReply),
		closed:  make(chan struct{}),
	}

	_, err = tr.call(context.Background(), "eth_chainId")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrTransportClosed,
		"write failures must classify as ErrTransportClosed so the caller redials")

	select {
	case <-tr.closed:
	default:
		t.Fatal("a failed write must shut the transport down, not leave it half-alive")
	}
}
