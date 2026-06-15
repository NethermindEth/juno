package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/l1/eth/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWS_UnaryCall(t *testing.T) {
	srv := client.NewTestServer(t)
	srv.SetHandler(func(req client.TestRequest) (any, *client.TestRPCError) {
		require.Equal(t, "eth_chainId", req.Method)
		return "0x539", nil
	})

	cli, err := client.New(t.Context(), srv.WSURL())
	require.NoError(t, err)
	t.Cleanup(cli.Close)

	id, err := cli.ChainID(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "1337", id.String())
}

func TestWS_SubscribeReceivesLogs(t *testing.T) {
	const subID = "0x1a2b3c"
	srv := client.NewTestServer(t)
	srv.SetHandler(func(req client.TestRequest) (any, *client.TestRPCError) {
		switch req.Method {
		case "eth_subscribe":
			require.GreaterOrEqual(t, len(req.Params), 1)
			return subID, nil
		case "eth_unsubscribe":
			return true, nil
		}
		return nil, &client.TestRPCError{Code: -32601, Message: req.Method}
	})

	cli, err := client.New(t.Context(), srv.WSURL())
	require.NoError(t, err)
	t.Cleanup(cli.Close)

	sink := make(chan *eth.Log, 4)
	sub, err := cli.SubscribeLogs(t.Context(), client.FilterQuery{}, sink)
	require.NoError(t, err)
	defer sub.Unsubscribe()

	// Push two log notifications.
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

	// Err() must NOT have fired yet.
	select {
	case err, open := <-sub.Err():
		t.Fatalf("Err() fired unexpectedly: err=%v open=%v", err, open)
	default:
	}
}

func TestWS_SubscribeServerError(t *testing.T) {
	srv := client.NewTestServer(t)
	srv.SetHandler(func(req client.TestRequest) (any, *client.TestRPCError) {
		return nil, &client.TestRPCError{Code: -32601, Message: "method not supported"}
	})
	cli, err := client.New(t.Context(), srv.WSURL())
	require.NoError(t, err)
	t.Cleanup(cli.Close)

	sink := make(chan *eth.Log, 1)
	_, err = cli.SubscribeLogs(t.Context(), client.FilterQuery{}, sink)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "subscribe to logs")
	assert.Contains(t, err.Error(), "method not supported")
}

func TestWS_ServerKillsConnection(t *testing.T) {
	const subID = "0xdeadbeef"
	srv := client.NewTestServer(t)
	srv.SetHandler(func(req client.TestRequest) (any, *client.TestRPCError) {
		require.Equal(t, "eth_subscribe", req.Method)
		return subID, nil
	})

	cli, err := client.New(t.Context(), srv.WSURL())
	require.NoError(t, err)
	t.Cleanup(cli.Close)

	sink := make(chan *eth.Log, 1)
	sub, err := cli.SubscribeLogs(t.Context(), client.FilterQuery{}, sink)
	require.NoError(t, err)

	// Kill the connection; Err() must fire.
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
	srv := client.NewTestServer(t)
	srv.SetHandler(func(req client.TestRequest) (any, *client.TestRPCError) {
		switch req.Method {
		case "eth_subscribe":
			return subID, nil
		case "eth_unsubscribe":
			sawUnsub.Store(true)
			return true, nil
		}
		return nil, &client.TestRPCError{Code: -32601, Message: req.Method}
	})

	cli, err := client.New(t.Context(), srv.WSURL())
	require.NoError(t, err)
	t.Cleanup(cli.Close)

	sink := make(chan *eth.Log, 1)
	sub, err := cli.SubscribeLogs(t.Context(), client.FilterQuery{}, sink)
	require.NoError(t, err)

	sub.Unsubscribe()

	// Err() closes on Unsubscribe.
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
	srv := client.NewTestServer(t)
	srv.SetHandler(func(req client.TestRequest) (any, *client.TestRPCError) {
		return "0xfeed", nil
	})

	cli, err := client.New(t.Context(), srv.WSURL())
	require.NoError(t, err)

	sink := make(chan *eth.Log, 1)
	sub, err := cli.SubscribeLogs(t.Context(), client.FilterQuery{}, sink)
	require.NoError(t, err)

	cli.Close()

	select {
	case err := <-sub.Err():
		// On clean close the cause is ErrTransportClosed; with a torn
		// transport it may be a wrapped websocket error. Either is OK.
		assert.True(t, err == nil || errors.Is(err, client.ErrTransportClosed) || err != nil)
	case <-time.After(2 * time.Second):
		t.Fatal("Err() did not fire after client.Close")
	}
}

func TestWS_ContextCancelMidCall(t *testing.T) {
	srv := client.NewTestServer(t)
	// Handler that never returns a result (simulate a hung server).
	gate := make(chan struct{})
	t.Cleanup(func() { close(gate) })
	srv.SetHandler(func(req client.TestRequest) (any, *client.TestRPCError) {
		<-gate
		return "0x0", nil
	})

	cli, err := client.New(t.Context(), srv.WSURL())
	require.NoError(t, err)
	t.Cleanup(cli.Close)

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	_, err = cli.ChainID(ctx)
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled), "expected context.Canceled, got %v", err)
}

// TestWS_SubscribeOmitsBlockRange is a regression test for the live-logs
// shape sent on eth_subscribe. Geth treats an explicit toBlock=0 as a
// bounded historical filter that terminates at block 0 — so an empty
// FilterQuery MUST serialise without fromBlock/toBlock keys, otherwise
// no live LogStateUpdate events ever reach the node.
//
// This is the unit-level guard for the bug; the manual Sepolia smoke is
// still the strongest end-to-end check.
func TestWS_SubscribeOmitsBlockRange(t *testing.T) {
	const subID = "0xfeed"
	type capturedSub struct {
		params []json.RawMessage
	}
	var captured atomic.Pointer[capturedSub]

	srv := client.NewTestServer(t)
	srv.SetHandler(func(req client.TestRequest) (any, *client.TestRPCError) {
		if req.Method == "eth_subscribe" {
			captured.Store(&capturedSub{params: req.Params})
			return subID, nil
		}
		if req.Method == "eth_unsubscribe" {
			return true, nil
		}
		return nil, &client.TestRPCError{Code: -32601, Message: req.Method}
	})

	cli, err := client.New(t.Context(), srv.WSURL())
	require.NoError(t, err)
	t.Cleanup(cli.Close)

	sink := make(chan *eth.Log, 1)
	sub, err := cli.SubscribeLogs(t.Context(), client.FilterQuery{}, sink)
	require.NoError(t, err)
	defer sub.Unsubscribe()

	got := captured.Load()
	require.NotNil(t, got, "eth_subscribe was never received by the test server")
	require.Len(t, got.params, 2, `expected ["logs", <filter>] params`)

	var filter map[string]any
	require.NoError(t, json.Unmarshal(got.params[1], &filter))
	_, hasFrom := filter["fromBlock"]
	assert.False(t, hasFrom,
		`eth_subscribe filter must omit "fromBlock" for a live-logs subscription; got %v`,
		filter,
	)
	_, hasTo := filter["toBlock"]
	assert.False(t, hasTo,
		`eth_subscribe filter must omit "toBlock" for a live-logs subscription; got %v`,
		filter,
	)
}

// receiveLogs drains up to n logs from sink with a timeout. Returns
// what it got; the caller asserts count and contents.
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
