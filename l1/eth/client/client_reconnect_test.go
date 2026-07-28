package client_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/l1/eth/client"
	"github.com/NethermindEth/juno/l1/internal/clienttest"
	"github.com/stretchr/testify/require"
)

func TestClient_RedialsAfterTransportDrop(t *testing.T) {
	srv := clienttest.NewTestServer(t)
	srv.SetHandler(func(req clienttest.TestRequest) (any, *clienttest.TestRPCError) {
		if req.Method == "eth_chainId" {
			return "0x539", nil
		}
		return nil, &clienttest.TestRPCError{Code: -32601, Message: req.Method}
	})

	cli, err := client.New(t.Context(), srv.WSURL())
	require.NoError(t, err)
	t.Cleanup(cli.Close)

	id, err := cli.ChainID(t.Context())
	require.NoError(t, err)
	require.Equal(t, "1337", id.String())

	srv.KillWSConns()

	// The transport shuts down asynchronously; early probes may still race it.
	require.Eventually(t, func() bool {
		_, err := cli.ChainID(t.Context())
		return err == nil
	}, 2*time.Second, 20*time.Millisecond,
		"ChainID should succeed after a transport drop via redial")
}

// TestClient_DroppingCallRedials verifies the call that observes the drop is
// itself retried on a fresh transport, not just later ones.
func TestClient_DroppingCallRedials(t *testing.T) {
	var callCount atomic.Int64
	firstCallStarted := make(chan struct{})
	releaseFirstCall := make(chan struct{})

	srv := clienttest.NewTestServer(t)
	srv.SetHandler(func(req clienttest.TestRequest) (any, *clienttest.TestRPCError) {
		if req.Method != "eth_chainId" {
			return nil, &clienttest.TestRPCError{Code: -32601, Message: req.Method}
		}
		if callCount.Add(1) == 1 {
			close(firstCallStarted)
			<-releaseFirstCall
		}
		return "0x539", nil
	})

	cli, err := client.New(t.Context(), srv.WSURL())
	require.NoError(t, err)
	t.Cleanup(cli.Close)

	done := make(chan error, 1)
	go func() {
		_, err := cli.ChainID(t.Context())
		done <- err
	}()

	select {
	case <-firstCallStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("handler never received the first call")
	}
	srv.KillWSConns()
	close(releaseFirstCall)

	select {
	case err := <-done:
		require.NoError(t, err, "the dropping call must redial and retry")
		require.GreaterOrEqual(t, callCount.Load(), int64(2))
	case <-time.After(5 * time.Second):
		t.Fatal("dropping call did not return")
	}
}

// TestClient_ConcurrentRedialsShareOneDial pins the single-flight contract:
// N callers observing the same dead transport must produce one new connection.
func TestClient_ConcurrentRedialsShareOneDial(t *testing.T) {
	srv := clienttest.NewTestServer(t)
	srv.SetHandler(func(req clienttest.TestRequest) (any, *clienttest.TestRPCError) {
		if req.Method == "eth_chainId" {
			return "0x1", nil
		}
		return nil, &clienttest.TestRPCError{Code: -32601, Message: req.Method}
	})

	cli, err := client.New(t.Context(), srv.WSURL())
	require.NoError(t, err)
	t.Cleanup(cli.Close)

	_, err = cli.ChainID(t.Context())
	require.NoError(t, err)

	srv.KillWSConns()

	const callers = 8
	errCh := make(chan error, callers)
	for range callers {
		go func() {
			// Retry through the async teardown window; each eventual success
			// must ride the same redialed connection.
			deadline := time.After(2 * time.Second)
			for {
				_, err := cli.ChainID(t.Context())
				if err == nil {
					errCh <- nil
					return
				}
				select {
				case <-deadline:
					errCh <- err
					return
				case <-time.After(10 * time.Millisecond):
				}
			}
		}()
	}
	for range callers {
		require.NoError(t, <-errCh)
	}
	require.Equal(t, 1, srv.WSConnCount(),
		"concurrent redials must coalesce into a single new connection")
}

// TestClient_DialFailuresDontHangCallers verifies callers racing a redial
// against a dead endpoint all fail promptly, sharing the flight's outcome.
func TestClient_DialFailuresDontHangCallers(t *testing.T) {
	srv := clienttest.NewTestServer(t)
	cli, err := client.New(t.Context(), srv.WSURL())
	require.NoError(t, err)
	t.Cleanup(cli.Close)

	srv.Close() // endpoint gone: transports die and every redial must fail

	const callers = 8
	done := make(chan error, callers)
	for range callers {
		go func() {
			_, err := cli.ChainID(t.Context())
			done <- err
		}()
	}
	for range callers {
		select {
		case err := <-done:
			require.Error(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("caller hung on a failing redial")
		}
	}
}

// TestClient_CloseIsTerminal pins that Close stops redials for good.
func TestClient_CloseIsTerminal(t *testing.T) {
	srv := clienttest.NewTestServer(t)
	srv.SetHandler(func(req clienttest.TestRequest) (any, *clienttest.TestRPCError) {
		return "0x1", nil
	})
	cli, err := client.New(t.Context(), srv.WSURL())
	require.NoError(t, err)

	_, err = cli.ChainID(t.Context())
	require.NoError(t, err)

	cli.Close()
	cli.Close() // idempotent

	_, err = cli.ChainID(t.Context())
	require.ErrorIs(t, err, client.ErrClosed)
	require.Eventually(t, func() bool { return srv.WSConnCount() == 0 },
		2*time.Second, 10*time.Millisecond,
		"Close must drop the connection and must not redial")
}

// TestClient_SubscribeRedialsAfterDrop verifies subscribing works on a fresh
// transport after the previous one died.
func TestClient_SubscribeRedialsAfterDrop(t *testing.T) {
	const subID = "0x51ab"
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

	cli, err := client.New(t.Context(), srv.WSURL())
	require.NoError(t, err)
	t.Cleanup(cli.Close)

	srv.KillWSConns()

	sink := make(chan *eth.Log, 1)
	var sub client.Subscription
	require.Eventually(t, func() bool {
		s, err := cli.SubscribeLogs(t.Context(), client.FilterQuery{}, sink)
		if err != nil {
			return false
		}
		sub = s
		return true
	}, 2*time.Second, 10*time.Millisecond, "subscribe must succeed via redial")
	defer sub.Unsubscribe()

	require.NoError(t, srv.PushNotification(t.Context(), subID, map[string]any{
		"blockNumber": "0x10",
		"removed":     false,
	}))
	got := receiveLogs(t, sink, 1, 2*time.Second)
	require.Len(t, got, 1)
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
