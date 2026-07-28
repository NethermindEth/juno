package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

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
