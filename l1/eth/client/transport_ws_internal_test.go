// White-box tests. These two assertions cannot be made through the public API:
// a dead conn triggers both the write path and readLoop's shutdown, so a
// black-box test can't pin WHICH path classified the error, and pendingSubs
// retention is only observable by inspecting the map itself.
package client

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/l1/eth/client/clienttest"
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
		conn:         conn,
		pending:      make(map[uint64]chan rpcReply),
		pendingSubs:  make(map[uint64]*wsLogSub),
		orphanedSubs: make(map[uint64]struct{}),
		subs:         make(map[string]*wsLogSub),
		pingReset:    make(chan struct{}, 1),
		closed:       make(chan struct{}),
	}

	sub := &wsLogSub{
		transport: tr,
		logCh:     make(chan json.RawMessage, 1),
		errCh:     make(chan error, 1),
		closed:    make(chan struct{}),
	}
	tr.subs["0xabc"] = sub

	_, err = tr.call(context.Background(), "eth_chainId")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrTransportClosed,
		"write failures must classify as ErrTransportClosed so the caller redials")

	select {
	case <-tr.closed:
	default:
		t.Fatal("a failed write must shut the transport down, not leave it half-alive")
	}
	select {
	case errOut := <-sub.Err():
		require.ErrorIs(t, errOut, ErrTransportClosed,
			"active subscriptions must fail promptly on write failure")
	default:
		t.Fatal("active subscription was not failed on write failure")
	}
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

	cli, err := New(t.Context(), srv.WSURL())
	require.NoError(t, err)
	t.Cleanup(cli.Close)

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		<-received
		cancel()
	}()
	sink := make(chan *eth.Log, 1)
	_, err = cli.SubscribeLogs(ctx, FilterQuery{}, sink)
	require.ErrorIs(t, err, context.Canceled)

	cli.tr.mu.Lock()
	retained := len(cli.tr.pendingSubs)
	cli.tr.mu.Unlock()
	require.Zero(t, retained,
		"cancelled subscribe must not retain its wsLogSub in pendingSubs")
}
