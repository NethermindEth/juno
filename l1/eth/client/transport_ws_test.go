package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/l1/eth/client"
	"github.com/NethermindEth/juno/l1/internal/clienttest"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T, srv *clienttest.TestServer, opts ...client.Option) *client.Client {
	t.Helper()
	opts = append([]client.Option{client.WithLogger(log.NewNopZapLogger())}, opts...)
	cli, err := client.New(t.Context(), srv.WSURL(), opts...)
	require.NoError(t, err)
	t.Cleanup(cli.Close)
	return cli
}

func TestWS_UnaryCall(t *testing.T) {
	srv := clienttest.NewTestServer(t)
	srv.SetHandler(func(req clienttest.TestRequest) (any, *clienttest.TestRPCError) {
		assert.Equal(t, "eth_chainId", req.Method)
		return "0x539", nil
	})

	cli := newTestClient(t, srv)

	raw, err := cli.Call(t.Context(), "eth_chainId")
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

	cli := newTestClient(t, srv)

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		<-received
		cancel()
	}()
	_, err := cli.Call(ctx, "eth_chainId")
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled), "expected context.Canceled, got %v", err)
}

func TestWS_PingsServerPeriodically(t *testing.T) {
	srv := clienttest.NewTestServer(t)
	newTestClient(t, srv, client.WithPingConfig(20*time.Millisecond, time.Second))

	require.Eventually(t, func() bool {
		return srv.PingsReceived() >= 3
	}, 2*time.Second, 10*time.Millisecond,
		"expected >= 3 pings within window; got %d", srv.PingsReceived())
}

func TestWS_UnansweredPingClosesConn(t *testing.T) {
	srv := clienttest.NewTestServer(t)
	srv.SetDropPings(true)
	cli := newTestClient(t, srv, client.WithPingConfig(20*time.Millisecond, 50*time.Millisecond))

	require.Eventually(t, func() bool { return srv.WSConnCount() == 0 },
		2*time.Second, 10*time.Millisecond, "conn not closed after ping timeout")
	_, err := cli.Call(t.Context(), "eth_chainId")
	require.ErrorIs(t, err, client.ErrTransportClosed)
}

func TestWS_MalformedFramesDoNotBreakClient(t *testing.T) {
	srv := clienttest.NewTestServer(t)
	srv.SetHandler(func(_ clienttest.TestRequest) (any, *clienttest.TestRPCError) {
		return "0x1", nil
	})

	cli := newTestClient(t, srv)

	frames := [][]byte{
		// Unparseable top-level JSON.
		[]byte(`{not json`),
		// No id and an unknown method.
		[]byte(`{"jsonrpc":"2.0","method":"unknown_method"}`),
		// Response with a non-numeric id.
		[]byte(`{"jsonrpc":"2.0","id":"not-a-number","result":"0x1"}`),
		// Response with a numeric id that doesn't match any in-flight call.
		[]byte(`{"jsonrpc":"2.0","id":999999,"result":"0x1"}`),
		// Notification for an unknown subscription id.
		[]byte(`{"jsonrpc":"2.0","method":"eth_subscription",` +
			`"params":{"subscription":"0xdead","result":{}}}`),
		// Notification with broken envelope (decode fails on params).
		[]byte(`{"jsonrpc":"2.0","method":"eth_subscription","params":"oops"}`),
		// Truncated response.
		[]byte(`{"jsonrpc":"2.0","id":1,"result":`),
	}
	for _, f := range frames {
		require.NoError(t, srv.PushRawFrame(t.Context(), f),
			"server-side write should not fail")
	}

	raw, err := cli.Call(t.Context(), "eth_chainId")
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

	cli := newTestClient(t, srv)

	const goroutines, callsEach = 8, 25
	var wg sync.WaitGroup
	errCh := make(chan error, goroutines*callsEach)
	for g := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range callsEach {
				tag := fmt.Sprintf("0x%x", uint64(g*callsEach+i+1))
				raw, err := cli.Call(t.Context(), "eth_getBlockByNumber", tag)
				if err != nil {
					errCh <- err
					return
				}
				if want := fmt.Sprintf("%q", tag); string(raw) != want {
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
// client closes with calls in flight: each returns client.ErrTransportClosed.
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

	cli := newTestClient(t, srv)

	const callers = 16
	done := make(chan error, callers)
	for range callers {
		go func() {
			_, err := cli.Call(t.Context(), "eth_chainId")
			done <- err
		}()
	}

	<-received // at least one call is in the server's hands
	cli.Close()

	for range callers {
		select {
		case err := <-done:
			require.Error(t, err)
			require.ErrorIs(t, err, client.ErrTransportClosed)
		case <-time.After(5 * time.Second):
			t.Fatal("caller hung after Close")
		}
	}
}

func TestWS_NewFailsWhenServerUnreachable(t *testing.T) {
	srv := clienttest.NewTestServer(t)
	url := srv.WSURL()
	srv.Close()

	_, err := client.New(t.Context(), url, client.WithDialTimeout(time.Second))
	require.Error(t, err)
}
