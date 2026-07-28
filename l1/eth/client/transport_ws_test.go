package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/l1/internal/clienttest"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestTransport(t *testing.T, srv *clienttest.TestServer, opts ...Option) *wsTransport {
	t.Helper()
	o := options{logger: log.NewNopZapLogger()}
	for _, opt := range opts {
		opt(&o)
	}
	tr, err := dialWS(t.Context(), srv.WSURL(), o)
	require.NoError(t, err)
	t.Cleanup(tr.close)
	return tr
}

func TestWS_UnaryCall(t *testing.T) {
	srv := clienttest.NewTestServer(t)
	srv.SetHandler(func(req clienttest.TestRequest) (any, *clienttest.TestRPCError) {
		require.Equal(t, "eth_chainId", req.Method)
		return "0x539", nil
	})

	tr := newTestTransport(t, srv)

	raw, err := tr.call(t.Context(), "eth_chainId")
	require.NoError(t, err)
	assert.Equal(t, `"0x539"`, string(raw))
}

func TestWS_ContextCancelMidCall(t *testing.T) {
	srv := clienttest.NewTestServer(t)
	// Handler signals on arrival then blocks, so the caller cancels with the
	// call reliably in flight - no wall-clock guessing.
	received := make(chan struct{})
	gate := make(chan struct{})
	t.Cleanup(func() { close(gate) })
	srv.SetHandler(func(req clienttest.TestRequest) (any, *clienttest.TestRPCError) {
		close(received)
		<-gate
		return "0x0", nil
	})

	tr := newTestTransport(t, srv)

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		<-received
		cancel()
	}()
	_, err := tr.call(ctx, "eth_chainId")
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled), "expected context.Canceled, got %v", err)
}

func TestWS_PingLoopFires(t *testing.T) {
	srv := clienttest.NewTestServer(t)
	tr := newTestTransport(t, srv, WithPingConfig(20*time.Millisecond, time.Second))
	_ = tr

	require.Eventually(t, func() bool {
		return srv.PingsReceived() >= 3
	}, 2*time.Second, 10*time.Millisecond,
		"expected >= 3 pings within window; got %d", srv.PingsReceived())
}

func TestWS_PingTimeoutClosesTransport(t *testing.T) {
	srv := clienttest.NewTestServer(t)
	srv.SetDropPings(true)
	tr := newTestTransport(t, srv, WithPingConfig(20*time.Millisecond, 50*time.Millisecond))

	select {
	case <-tr.closed:
	case <-time.After(2 * time.Second):
		t.Fatal("transport did not shut down after ping timeout")
	}
	_, err := tr.call(t.Context(), "eth_chainId")
	require.ErrorIs(t, err, ErrTransportClosed)
}

func TestWS_DispatchDropsMalformedFrames(t *testing.T) {
	const subID = "0xabc"
	srv := clienttest.NewTestServer(t)
	srv.SetHandler(func(req clienttest.TestRequest) (any, *clienttest.TestRPCError) {
		switch req.Method {
		case "eth_subscribe":
			return subID, nil
		case "eth_unsubscribe":
			return true, nil
		case "eth_chainId":
			return "0x1", nil
		}
		return nil, &clienttest.TestRPCError{Code: -32601, Message: req.Method}
	})

	tr := newTestTransport(t, srv)

	frames := [][]byte{
		// Unparseable top-level JSON.
		[]byte(`{not json`),
		// No id and no recognised method → "drop frame" branch.
		[]byte(`{"jsonrpc":"2.0","method":"unknown_method"}`),
		// Response with a string id that isn't numeric — parseResponseID errors.
		[]byte(`{"jsonrpc":"2.0","id":"not-a-number","result":"0x1"}`),
		// Response with a numeric id that doesn't match any in-flight call.
		[]byte(`{"jsonrpc":"2.0","id":999999,"result":"0x1"}`),
		// Notification for an unknown subscription id.
		[]byte(`{"jsonrpc":"2.0","method":"eth_subscription",` +
			`"params":{"subscription":"0xdead","result":{}}}`),
		// Notification with broken envelope (decode fails on params).
		[]byte(`{"jsonrpc":"2.0","method":"eth_subscription","params":"oops"}`),
		// Response with malformed body (id present, but result not decodable as JSON).
		[]byte(`{"jsonrpc":"2.0","id":1,"result":`),
	}
	for _, f := range frames {
		require.NoError(t, srv.PushRawFrame(t.Context(), f),
			"server-side write should not fail")
	}

	raw, err := tr.call(t.Context(), "eth_chainId")
	require.NoError(t, err, "transport must survive every malformed frame")
	assert.Equal(t, `"0x1"`, string(raw))
}

// TestWS_ConcurrentCallsGetTheirOwnReplies drives the id-routing table with
// parallel callers; a mis-keyed reply surfaces as a caller receiving another
// caller's block number.
func TestWS_ConcurrentCallsGetTheirOwnReplies(t *testing.T) {
	srv := clienttest.NewTestServer(t)
	srv.SetHandler(func(req clienttest.TestRequest) (any, *clienttest.TestRPCError) {
		if req.Method != "eth_getBlockByNumber" {
			return nil, &clienttest.TestRPCError{Code: -32601, Message: req.Method}
		}
		// Echo the requested tag back as the block number, correlating
		// each reply with exactly one call.
		var tag string
		if err := json.Unmarshal(req.Params[0], &tag); err != nil {
			return nil, &clienttest.TestRPCError{Code: -32602, Message: err.Error()}
		}
		return tag, nil
	})

	tr := newTestTransport(t, srv)

	const goroutines, callsEach = 8, 25
	var wg sync.WaitGroup
	errCh := make(chan error, goroutines*callsEach)
	for g := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range callsEach {
				want := fmt.Sprintf("%q", fmt.Sprintf("0x%x", uint64(g*callsEach+i+1)))
				raw, err := tr.call(t.Context(), "eth_getBlockByNumber", fmt.Sprintf("0x%x", uint64(g*callsEach+i+1)))
				if err != nil {
					errCh <- err
					return
				}
				if string(raw) != want {
					errCh <- fmt.Errorf("cross-wired reply: got %s, want %s", raw, want)
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
}

// TestWS_CloseRacesInFlightCalls verifies no caller hangs or panics when the
// client closes with calls in flight: each gets a result or ErrTransportClosed.
func TestWS_CloseRacesInFlightCalls(t *testing.T) {
	received := make(chan struct{}, 1)
	gate := make(chan struct{})
	t.Cleanup(func() { close(gate) })
	srv := clienttest.NewTestServer(t)
	srv.SetHandler(func(_ clienttest.TestRequest) (any, *clienttest.TestRPCError) {
		select {
		case received <- struct{}{}:
		default:
		}
		<-gate
		return "0x1", nil
	})

	tr := newTestTransport(t, srv)

	const callers = 16
	done := make(chan error, callers)
	for range callers {
		go func() {
			_, err := tr.call(t.Context(), "eth_chainId")
			done <- err
		}()
	}

	<-received // at least one call is in the server's hands
	tr.close()

	for range callers {
		select {
		case err := <-done:
			require.Error(t, err)
			require.ErrorIs(t, err, ErrTransportClosed)
		case <-time.After(5 * time.Second):
			t.Fatal("caller hung after Close")
		}
	}
}

func TestWS_SubscribeReceivesLogs(t *testing.T) {
	const subID = "0x1a2b3c"
	srv := clienttest.NewTestServer(t)
	srv.SetHandler(func(req clienttest.TestRequest) (any, *clienttest.TestRPCError) {
		switch req.Method {
		case "eth_subscribe":
			require.GreaterOrEqual(t, len(req.Params), 1)
			return subID, nil
		case "eth_unsubscribe":
			return true, nil
		}
		return nil, &clienttest.TestRPCError{Code: -32601, Message: req.Method}
	})

	tr := newTestTransport(t, srv)

	sink := make(chan *eth.Log, 4)
	q := FilterQuery{Topics: [][]eth.Hash{{eth.HashFromString(
		"0xdb80dd488acf86d17c747445b0eabb5d57c541d3bd7b6b87af987858e5066b2b",
	)}}}
	sub, err := tr.subscribeLogs(t.Context(), q, sink)
	require.NoError(t, err)
	defer sub.Unsubscribe()

	for _, bnHex := range []string{"0x10", "0x11"} {
		require.NoError(t, srv.PushNotification(t.Context(), subID, map[string]any{
			"topics":      []string{"0xdb80dd488acf86d17c747445b0eabb5d57c541d3bd7b6b87af987858e5066b2b"},
			"data":        "0x",
			"blockNumber": bnHex,
			"removed":     false,
		}))
	}

	got := receiveLogs(t, sink, 2, 2*time.Second)
	require.Len(t, got, 2)
	assert.Equal(t, uint64(0x10), uint64(got[0].BlockNumber))
	assert.Equal(t, uint64(0x11), uint64(got[1].BlockNumber))

	select {
	case err, open := <-sub.Err():
		t.Fatalf("Err() fired unexpectedly: err=%v open=%v", err, open)
	default:
	}
}

func TestWS_SubscribeServerError(t *testing.T) {
	srv := clienttest.NewTestServer(t)
	srv.SetHandler(func(req clienttest.TestRequest) (any, *clienttest.TestRPCError) {
		return nil, &clienttest.TestRPCError{Code: -32601, Message: "method not supported"}
	})
	tr := newTestTransport(t, srv)

	sink := make(chan *eth.Log, 1)
	_, err := tr.subscribeLogs(t.Context(), FilterQuery{}, sink)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "subscribing to logs")
	assert.Contains(t, err.Error(), "method not supported")
}

func TestWS_ServerKillsConnection(t *testing.T) {
	const subID = "0xdeadbeef"
	srv := clienttest.NewTestServer(t)
	srv.SetHandler(func(req clienttest.TestRequest) (any, *clienttest.TestRPCError) {
		require.Equal(t, "eth_subscribe", req.Method)
		return subID, nil
	})

	tr := newTestTransport(t, srv)

	sink := make(chan *eth.Log, 1)
	sub, err := tr.subscribeLogs(t.Context(), FilterQuery{}, sink)
	require.NoError(t, err)

	srv.KillWSConns()

	select {
	case err := <-sub.Err():
		assert.Error(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Err() did not fire after server killed the connection")
	}
}

func TestWS_UnsubscribeIssuesCall(t *testing.T) {
	const subID = "0xabc"
	var sawUnsub atomic.Bool
	srv := clienttest.NewTestServer(t)
	srv.SetHandler(func(req clienttest.TestRequest) (any, *clienttest.TestRPCError) {
		switch req.Method {
		case "eth_subscribe":
			return subID, nil
		case "eth_unsubscribe":
			sawUnsub.Store(true)
			return true, nil
		}
		return nil, &clienttest.TestRPCError{Code: -32601, Message: req.Method}
	})

	tr := newTestTransport(t, srv)

	sink := make(chan *eth.Log, 1)
	sub, err := tr.subscribeLogs(t.Context(), FilterQuery{}, sink)
	require.NoError(t, err)

	sub.Unsubscribe()

	select {
	case _, open := <-sub.Err():
		assert.False(t, open, "Err() should be closed after Unsubscribe")
	case <-time.After(time.Second):
		t.Fatal("Err() did not close after Unsubscribe")
	}
	require.Eventually(t, sawUnsub.Load, 2*time.Second, 10*time.Millisecond,
		"server never received eth_unsubscribe")
}

func TestWS_ClientCloseFailsActiveSubscriptions(t *testing.T) {
	srv := clienttest.NewTestServer(t)
	srv.SetHandler(func(req clienttest.TestRequest) (any, *clienttest.TestRPCError) {
		return "0xfeed", nil
	})

	tr := newTestTransport(t, srv)

	sink := make(chan *eth.Log, 1)
	sub, err := tr.subscribeLogs(t.Context(), FilterQuery{}, sink)
	require.NoError(t, err)

	tr.close()

	select {
	case err := <-sub.Err():
		assert.ErrorIs(t, err, ErrTransportClosed)
	case <-time.After(2 * time.Second):
		t.Fatal("Err() did not fire after client.Close")
	}
}

func TestWS_SubscribeOmitsBlockRange(t *testing.T) {
	const subID = "0xfeed"
	type capturedSub struct {
		params []json.RawMessage
	}
	var captured atomic.Pointer[capturedSub]

	srv := clienttest.NewTestServer(t)
	srv.SetHandler(func(req clienttest.TestRequest) (any, *clienttest.TestRPCError) {
		if req.Method == "eth_subscribe" {
			captured.Store(&capturedSub{params: req.Params})
			return subID, nil
		}
		if req.Method == "eth_unsubscribe" {
			return true, nil
		}
		return nil, &clienttest.TestRPCError{Code: -32601, Message: req.Method}
	})

	tr := newTestTransport(t, srv)

	sink := make(chan *eth.Log, 1)
	sub, err := tr.subscribeLogs(t.Context(), FilterQuery{}, sink)
	require.NoError(t, err)
	defer sub.Unsubscribe()

	got := captured.Load()
	require.NotNil(t, got, "eth_subscribe was never received by the test server")
	require.Len(t, got.params, 2, `expected ["logs", <filter>] params`)

	var filter map[string]any
	require.NoError(t, json.Unmarshal(got.params[1], &filter))
	_, hasFrom := filter["fromBlock"]
	assert.False(
		t, hasFrom,
		`eth_subscribe filter must omit "fromBlock" for a live-logs subscription; got %v`,
		filter,
	)
	_, hasTo := filter["toBlock"]
	assert.False(
		t, hasTo,
		`eth_subscribe filter must omit "toBlock" for a live-logs subscription; got %v`,
		filter,
	)
}

func TestWS_SubscribeCtxCancelBeforeReplyReleasesServerSub(t *testing.T) {
	const subID = "0xabc"
	release := make(chan struct{})
	gotSubscribe := make(chan struct{}, 1)
	gotUnsubscribe := make(chan string, 1)

	srv := clienttest.NewTestServer(t)
	srv.SetHandler(func(req clienttest.TestRequest) (any, *clienttest.TestRPCError) {
		switch req.Method {
		case "eth_subscribe":
			gotSubscribe <- struct{}{}
			<-release // hold the reply until the test cancels the ctx
			return subID, nil
		case "eth_unsubscribe":
			var id string
			_ = json.Unmarshal(req.Params[0], &id)
			gotUnsubscribe <- id
			return true, nil
		}
		return nil, &clienttest.TestRPCError{Code: -32601, Message: req.Method}
	})

	tr := newTestTransport(t, srv)

	ctx, cancel := context.WithCancel(context.Background())
	sink := make(chan *eth.Log, 1)
	subErr := make(chan error, 1)
	go func() {
		_, e := tr.subscribeLogs(ctx, FilterQuery{}, sink)
		subErr <- e
	}()

	// Cancel while the server holds the reply, forcing the caller onto ctx.Done().
	<-gotSubscribe
	cancel()
	require.ErrorIs(t, <-subErr, context.Canceled)

	// Releasing the abandoned reply must trigger the unsubscribe.
	close(release)
	select {
	case id := <-gotUnsubscribe:
		require.Equal(t, subID, id)
	case <-time.After(2 * time.Second):
		t.Fatal("transport did not release the orphaned server-side subscription")
	}
}

func TestWS_UnsubscribeToleratesServerRejection(t *testing.T) {
	const subID = "0xrej"
	var unsubAttempted atomic.Bool

	srv := clienttest.NewTestServer(t)
	srv.SetHandler(func(req clienttest.TestRequest) (any, *clienttest.TestRPCError) {
		switch req.Method {
		case "eth_subscribe":
			return subID, nil
		case "eth_unsubscribe":
			unsubAttempted.Store(true)
			return nil, &clienttest.TestRPCError{Code: -32000, Message: "subscription not found"}
		}
		return nil, &clienttest.TestRPCError{Code: -32601, Message: req.Method}
	})

	tr := newTestTransport(t, srv)

	sink := make(chan *eth.Log, 1)
	sub, err := tr.subscribeLogs(t.Context(), FilterQuery{}, sink)
	require.NoError(t, err)

	sub.Unsubscribe()
	require.True(t, unsubAttempted.Load(), "Unsubscribe must attempt eth_unsubscribe")
}

func TestWS_SubscribeDispatchDecodeFailure(t *testing.T) {
	const subID = "0xc0de"
	srv := clienttest.NewTestServer(t)
	srv.SetHandler(func(req clienttest.TestRequest) (any, *clienttest.TestRPCError) {
		switch req.Method {
		case "eth_subscribe":
			return subID, nil
		case "eth_unsubscribe":
			return true, nil
		}
		return nil, &clienttest.TestRPCError{Code: -32601, Message: req.Method}
	})

	tr := newTestTransport(t, srv)

	sink := make(chan *eth.Log, 1)
	sub, err := tr.subscribeLogs(t.Context(), FilterQuery{}, sink)
	require.NoError(t, err)
	defer sub.Unsubscribe()

	// topics expects an array of hex strings; a string forces the unmarshal to fail.
	require.NoError(t, srv.PushNotification(t.Context(), subID, map[string]any{
		"topics": "not-an-array",
	}))

	select {
	case errOut, open := <-sub.Err():
		assert.True(t, open, "Err() must deliver the cause before closing")
		require.Error(t, errOut)
		assert.Contains(t, errOut.Error(), "decoding log")
	case <-time.After(2 * time.Second):
		t.Fatal("Err() did not fire on undecodable notification payload")
	}
}

func TestWS_CallReturnsCtxErrAfterCancellation(t *testing.T) {
	received := make(chan struct{})
	gate := make(chan struct{})
	t.Cleanup(func() { close(gate) })
	srv := clienttest.NewTestServer(t)
	srv.SetHandler(func(_ clienttest.TestRequest) (any, *clienttest.TestRPCError) {
		close(received)
		<-gate
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
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled), "expected ctx.Canceled, got %v", err)
}

func TestWS_SlowSubscriberFailsInsteadOfStallingConn(t *testing.T) {
	const subID = "0x5109"
	srv := clienttest.NewTestServer(t)
	srv.SetHandler(func(req clienttest.TestRequest) (any, *clienttest.TestRPCError) {
		switch req.Method {
		case "eth_subscribe":
			return subID, nil
		case "eth_unsubscribe":
			return true, nil
		case "eth_chainId":
			return "0x1", nil
		}
		return nil, &clienttest.TestRPCError{Code: -32601, Message: req.Method}
	})

	tr := newTestTransport(t, srv)

	sink := make(chan *eth.Log)
	sub, err := tr.subscribeLogs(t.Context(), FilterQuery{}, sink)
	require.NoError(t, err)

	// 2x the internal buffer guarantees overflow.
	for range 128 {
		require.NoError(t, srv.PushNotification(t.Context(), subID, map[string]any{
			"topics":      []string{"0xdb80dd488acf86d17c747445b0eabb5d57c541d3bd7b6b87af987858e5066b2b"},
			"data":        "0x",
			"blockNumber": "0x10",
			"removed":     false,
		}))
	}

	select {
	case err := <-sub.Err():
		require.ErrorIs(t, err, ErrSubscriptionQueueOverflow)
	case <-time.After(2 * time.Second):
		t.Fatal("subscription Err() did not fire on queue overflow")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	raw, err := tr.call(ctx, "eth_chainId")
	require.NoError(t, err, "unary call stalled by a slow subscriber")
	assert.Equal(t, `"0x1"`, string(raw))
}

func receiveLogs(t *testing.T, sink <-chan *eth.Log, n int, timeout time.Duration) []*eth.Log {
	t.Helper()
	deadline := time.After(timeout)
	out := make([]*eth.Log, 0, n)
	for len(out) < n {
		select {
		case log := <-sink:
			out = append(out, log)
		case <-deadline:
			return out
		}
	}
	return out
}
