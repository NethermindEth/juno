// White-box tests. These two assertions cannot be made through the public API:
// a dead conn triggers both the write path and readLoop's shutdown, so a
// black-box test can't pin WHICH path classified the error, and pendingSubs
// retention is only observable by inspecting the map itself.
package client

import (
	"context"
	"testing"

	"github.com/NethermindEth/juno/l1/eth"
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

func TestWS_OrphanedSubsIsBounded(t *testing.T) {
	gate := make(chan struct{})
	t.Cleanup(func() { close(gate) })
	srv := clienttest.NewTestServer(t)
	srv.SetHandler(func(_ clienttest.TestRequest) (any, *clienttest.TestRPCError) {
		<-gate // never reply
		return nil, nil
	})

	tr := newTestTransport(t, srv)

	// A pre-cancelled ctx makes every subscribe write its frame and then
	// immediately abandon the call, orphaning the request id.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	sink := make(chan *eth.Log, 1)
	for range maxOrphanedSubs + 1 {
		_, err := tr.subscribeLogs(ctx, FilterQuery{}, sink)
		require.ErrorIs(t, err, context.Canceled)
	}

	tr.mu.Lock()
	orphans := len(tr.orphanedSubs)
	tr.mu.Unlock()
	require.LessOrEqual(t, orphans, maxOrphanedSubs,
		"orphanedSubs must not grow beyond its cap against an unresponsive server")
}

func TestWS_CancelledSubscribeDoesNotRetainPendingSub(t *testing.T) {
	received := make(chan struct{})
	gate := make(chan struct{})
	t.Cleanup(func() { close(gate) })
	srv := clienttest.NewTestServer(t)
	srv.SetHandler(func(req clienttest.TestRequest) (any, *clienttest.TestRPCError) {
		close(received)
		<-gate // hold the reply past the caller's cancellation
		return "0xfeed", nil
	})

	tr := newTestTransport(t, srv)

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		<-received
		cancel()
	}()
	sink := make(chan *eth.Log, 1)
	_, err := tr.subscribeLogs(ctx, FilterQuery{}, sink)
	require.ErrorIs(t, err, context.Canceled)

	tr.mu.Lock()
	retained := len(tr.pendingSubs)
	tr.mu.Unlock()
	require.Zero(t, retained,
		"cancelled subscribe must not retain its wsLogSub in pendingSubs")
}
