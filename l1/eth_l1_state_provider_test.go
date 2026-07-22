package l1_test

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/l1"
	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/l1/eth/client/clienttest"
	"github.com/NethermindEth/juno/l1/eth/contract"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ethRecordingListener captures every OnL1Call to verify the deferred-observe
// contract: the listener fires on both success and failure so error rates show
// in metrics.
type ethRecordingListener struct {
	mu    sync.Mutex
	calls []string // method names, in order
}

func (r *ethRecordingListener) OnNewL1Head(_ *core.L1Head) {}
func (r *ethRecordingListener) OnL1Call(method string, _ time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, method)
}

func (r *ethRecordingListener) Methods() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.calls))
	copy(out, r.calls)
	return out
}

// TestEthL1StateProvider_RedialsAfterTransportClosed is a regression target for
// the "subscribe to LogStateUpdate: ... transport closed" bug: a killed ws
// transport must auto-redial on the next call, else the provider is stuck
// returning ErrTransportClosed forever.
func TestEthL1StateProvider_RedialsAfterTransportClosed(t *testing.T) {
	srv := clienttest.NewTestServer(t)
	srv.SetHandler(func(req clienttest.TestRequest) (any, *clienttest.TestRPCError) {
		if req.Method == "eth_chainId" {
			return "0x539", nil
		}
		return nil, &clienttest.TestRPCError{Code: -32601, Message: req.Method}
	})

	s, err := l1.NewEthL1StateProvider(t.Context(), srv.WSURL(), eth.Address{})
	require.NoError(t, err)
	t.Cleanup(s.Close)

	id, err := s.ChainID(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "1337", id.String())

	srv.KillWSConns()

	// Poll until a redial succeeds; the transport shuts down asynchronously so
	// early probes may still return ErrTransportClosed.
	require.Eventually(t, func() bool {
		_, err := s.ChainID(t.Context())
		return err == nil
	}, 2*time.Second, 20*time.Millisecond,
		"ChainID should succeed after transport drop via auto-redial")
}

// TestEthL1StateProvider_DroppingCallRedials is the regression target for the
// shutdown-cause normalisation fix: a conn dying mid-call fans out the raw
// read/ping error, not ErrTransportClosed. Without normalisation withRetryOnClosed
// doesn't retry and the dropping call itself fails - fatal for one-shot callers
// (RPC TransactionReceipt) with no retry loop above them.
func TestEthL1StateProvider_DroppingCallRedials(t *testing.T) {
	var callCount atomic.Int64
	firstCallStarted := make(chan struct{})
	releaseFirstCall := make(chan struct{})

	srv := clienttest.NewTestServer(t)
	srv.SetHandler(func(req clienttest.TestRequest) (any, *clienttest.TestRPCError) {
		if req.Method != "eth_chainId" {
			return nil, &clienttest.TestRPCError{Code: -32601, Message: req.Method}
		}
		n := callCount.Add(1)
		if n == 1 {
			// Signal arrival then block; the conn is killed meanwhile so this
			// reply never reaches the client.
			close(firstCallStarted)
			<-releaseFirstCall
		}
		return "0x539", nil
	})

	s, err := l1.NewEthL1StateProvider(t.Context(), srv.WSURL(), eth.Address{})
	require.NoError(t, err)
	t.Cleanup(s.Close)

	type res struct {
		id  *big.Int
		err error
	}
	done := make(chan res, 1)
	go func() {
		id, err := s.ChainID(t.Context())
		done <- res{id, err}
	}()

	// Sever the conn while the first call is blocked in the handler, fanning
	// the read-error cause out to the in-flight call.
	select {
	case <-firstCallStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("handler never received the first eth_chainId call")
	}
	srv.KillWSConns()
	close(releaseFirstCall) // first handler unblocks; its write is a no-op (conn dead)

	select {
	case r := <-done:
		require.NoError(t, r.err,
			"dropping call must auto-redial; got: %v", r.err)
		assert.Equal(t, "1337", r.id.String())
		assert.GreaterOrEqual(t, callCount.Load(), int64(2),
			"redial must have invoked the handler a second time")
	case <-time.After(5 * time.Second):
		t.Fatal("dropping ChainID did not return via auto-redial")
	}
}

// TestEthL1StateProvider_AfterCloseReturnsErrClosed verifies Close is
// terminal: post-Close calls don't trigger another redial and instead
// return ErrClosed.
func TestEthL1StateProvider_AfterCloseReturnsErrClosed(t *testing.T) {
	srv := clienttest.NewTestServer(t)
	srv.SetHandler(func(req clienttest.TestRequest) (any, *clienttest.TestRPCError) {
		if req.Method == "eth_chainId" {
			return "0x1", nil
		}
		return nil, &clienttest.TestRPCError{Code: -32601, Message: req.Method}
	})

	s, err := l1.NewEthL1StateProvider(t.Context(), srv.WSURL(), eth.Address{})
	require.NoError(t, err)

	_, err = s.ChainID(t.Context())
	require.NoError(t, err)

	s.Close()

	_, err = s.ChainID(t.Context())
	require.Error(t, err)
	assert.ErrorIs(t, err, l1.ErrClosed)
}

// TestEthL1StateProvider_ListenerFiresOnErrorPath is a regression target: earlier
// code emitted OnL1Call only on success. It must fire on failure too so dashboards
// tell "L1 healthy and quiet" from "L1 broken".
func TestEthL1StateProvider_ListenerFiresOnErrorPath(t *testing.T) {
	srv := clienttest.NewTestServer(t)
	// Every method errors out so we exercise the failure branch.
	srv.SetHandler(func(_ clienttest.TestRequest) (any, *clienttest.TestRPCError) {
		return nil, &clienttest.TestRPCError{Code: -32000, Message: "boom"}
	})

	rec := &ethRecordingListener{}
	s, err := l1.NewEthL1StateProvider(t.Context(), srv.WSURL(), eth.Address{},
		l1.WithEthL1StateProviderListener(rec),
	)
	require.NoError(t, err)
	t.Cleanup(s.Close)

	_, _ = s.ChainID(t.Context())
	_, _ = s.LatestHeight(t.Context())
	_, _ = s.FinalisedHeight(t.Context())
	_, _ = s.TransactionReceipt(t.Context(), eth.Hash{})

	got := rec.Methods()
	want := []string{
		"eth_chainId",
		"eth_blockNumber",
		"eth_getBlockByNumber",
		"eth_getTransactionReceipt",
	}
	assert.Equal(t, want, got,
		"OnL1Call must fire on error paths so error rate is observable")
}

// TestEthL1StateProvider_CloseIsIdempotent — double-Close shouldn't panic
// or block, and shouldn't trigger a redial.
func TestEthL1StateProvider_CloseIsIdempotent(t *testing.T) {
	srv := clienttest.NewTestServer(t)
	s, err := l1.NewEthL1StateProvider(t.Context(), srv.WSURL(), eth.Address{})
	require.NoError(t, err)

	s.Close()
	s.Close()
}

// TestEthL1StateProvider_PreservesErrNotFound verifies eth.ErrNotFound survives
// the withRetryOnClosed helper so callers can still errors.Is it.
func TestEthL1StateProvider_PreservesErrNotFound(t *testing.T) {
	srv := clienttest.NewTestServer(t)
	srv.SetHandler(func(req clienttest.TestRequest) (any, *clienttest.TestRPCError) {
		if req.Method == "eth_getBlockByNumber" {
			return nil, nil // null result → ErrNotFound at the client layer
		}
		return nil, &clienttest.TestRPCError{Code: -32601, Message: req.Method}
	})

	s, err := l1.NewEthL1StateProvider(t.Context(), srv.WSURL(), eth.Address{})
	require.NoError(t, err)
	t.Cleanup(s.Close)

	_, err = s.FinalisedHeight(t.Context())
	require.Error(t, err)
	assert.True(t, errors.Is(err, eth.ErrNotFound),
		"FinalisedHeight must wrap eth.ErrNotFound so callers can errors.Is it; got: %v", err)
}

// stateUpdateLogJSON returns a JSON-RPC-shaped LogStateUpdate event. The 96-byte
// data section is the canonical globalRoot ‖ blockNumber (int256, low 8) ‖ blockHash.
func stateUpdateLogJSON(blockNumber, l1RefHeight uint64, removed bool) map[string]any {
	data := make([]byte, 96)
	// globalRoot — last byte distinguishes this event.
	data[31] = byte(blockNumber & 0xff)
	// blockNumber int256 — low 8 bytes.
	binary.BigEndian.PutUint64(data[56:64], blockNumber)
	// blockHash — last byte distinguishes this event.
	data[95] = byte((blockNumber >> 8) & 0xff)
	return map[string]any{
		"topics":      []string{contract.LogStateUpdateSigHash.Hex()},
		"data":        "0x" + hex.EncodeToString(data),
		"blockNumber": fmt.Sprintf("0x%x", l1RefHeight),
		"removed":     removed,
	}
}

// TestEthL1StateProvider_FilterStateUpdate_DecodesAndTranslates verifies the
// FilterStateUpdate path: eth_getLogs -> contract decoder -> chain-neutral
// StateUpdate, including the field mapping (Removed, L1RefHeight, BlockNumber).
func TestEthL1StateProvider_FilterStateUpdate_DecodesAndTranslates(t *testing.T) {
	srv := clienttest.NewTestServer(t)
	srv.SetHandler(func(req clienttest.TestRequest) (any, *clienttest.TestRPCError) {
		if req.Method == "eth_getLogs" {
			return []any{
				stateUpdateLogJSON(1, 1_000, false),
				stateUpdateLogJSON(2, 1_001, true),
			}, nil
		}
		return nil, &clienttest.TestRPCError{Code: -32601, Message: req.Method}
	})

	s, err := l1.NewEthL1StateProvider(t.Context(), srv.WSURL(), eth.Address{})
	require.NoError(t, err)
	t.Cleanup(s.Close)

	got, err := s.FilterStateUpdate(t.Context(), 100, 200)
	require.NoError(t, err)
	require.Len(t, got, 2)

	assert.Equal(t, uint64(1), got[0].L2BlockNumber)
	assert.Equal(t, uint64(1_000), got[0].L1RefHeight)
	assert.False(t, got[0].Removed)
	require.NotNil(t, got[0].L2BlockHash)
	require.NotNil(t, got[0].StateRoot)

	assert.Equal(t, uint64(2), got[1].L2BlockNumber)
	assert.Equal(t, uint64(1_001), got[1].L1RefHeight)
	assert.True(t, got[1].Removed, "Removed flag must round-trip from the raw log envelope")
}

// TestEthL1StateProvider_FilterStateUpdate_ErrorWrapsRange verifies the error
// includes the [from, to] range so operators can correlate it with L1 sync logs.
func TestEthL1StateProvider_FilterStateUpdate_ErrorWrapsRange(t *testing.T) {
	srv := clienttest.NewTestServer(t)
	srv.SetHandler(func(_ clienttest.TestRequest) (any, *clienttest.TestRPCError) {
		return nil, &clienttest.TestRPCError{Code: -32000, Message: "query timeout"}
	})

	s, err := l1.NewEthL1StateProvider(t.Context(), srv.WSURL(), eth.Address{})
	require.NoError(t, err)
	t.Cleanup(s.Close)

	_, err = s.FilterStateUpdate(t.Context(), 42, 99)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "[42,99]",
		"error must surface the requested range so operators can correlate")
	assert.Contains(t, err.Error(), "query timeout")
}

// TestEthL1StateProvider_WatchStateUpdate_DeliversDecoded verifies the live
// subscribe path: a pushed notification is decoded via contract.Decode and
// emitted as a chain-neutral StateUpdate.
func TestEthL1StateProvider_WatchStateUpdate_DeliversDecoded(t *testing.T) {
	const subID = "0xb10c"
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

	s, err := l1.NewEthL1StateProvider(t.Context(), srv.WSURL(), eth.Address{})
	require.NoError(t, err)
	t.Cleanup(s.Close)

	sink := make(chan *l1.StateUpdate, 4)
	sub, err := s.WatchStateUpdate(t.Context(), sink)
	require.NoError(t, err)
	defer sub.Unsubscribe()

	require.NoError(t, srv.PushNotification(
		t.Context(), subID, stateUpdateLogJSON(7, 2_000, false),
	))

	select {
	case su := <-sink:
		assert.Equal(t, uint64(7), su.L2BlockNumber)
		assert.Equal(t, uint64(2_000), su.L1RefHeight)
		assert.False(t, su.Removed)
	case <-time.After(2 * time.Second):
		t.Fatal("WatchStateUpdate did not deliver a decoded event")
	}

	// Err() must not have fired on the happy path.
	select {
	case err := <-sub.Err():
		t.Fatalf("Err() fired unexpectedly: %v", err)
	default:
	}
}

// TestEthL1StateProvider_WatchStateUpdate_UnsubscribeIsClean verifies Unsubscribe
// closes Err() with no spurious error and is idempotent (double call must not
// panic on already-closed channels).
func TestEthL1StateProvider_WatchStateUpdate_UnsubscribeIsClean(t *testing.T) {
	const subID = "0xbeef"
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

	s, err := l1.NewEthL1StateProvider(t.Context(), srv.WSURL(), eth.Address{})
	require.NoError(t, err)
	t.Cleanup(s.Close)

	sink := make(chan *l1.StateUpdate, 1)
	sub, err := s.WatchStateUpdate(t.Context(), sink)
	require.NoError(t, err)

	sub.Unsubscribe()
	sub.Unsubscribe() // idempotent

	// Err() must be closed after Unsubscribe (no cause).
	select {
	case errOut, open := <-sub.Err():
		assert.False(t, open, "Err() should be closed; got open with err=%v", errOut)
	case <-time.After(time.Second):
		t.Fatal("Err() did not close after Unsubscribe")
	}
}

// TestEthL1StateProvider_WatchStateUpdate_PropagatesInnerErr verifies forwarder.run()
// delivers the inner subscription's cause onto its own Err() - the failure the
// l1.Client redial loop reacts to.
func TestEthL1StateProvider_WatchStateUpdate_PropagatesInnerErr(t *testing.T) {
	const subID = "0xfa11"
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

	s, err := l1.NewEthL1StateProvider(t.Context(), srv.WSURL(), eth.Address{})
	require.NoError(t, err)
	t.Cleanup(s.Close)

	sink := make(chan *l1.StateUpdate, 1)
	sub, err := s.WatchStateUpdate(t.Context(), sink)
	require.NoError(t, err)
	defer sub.Unsubscribe()

	// Killing the conn must propagate the inner Err() to the caller-facing Err().
	srv.KillWSConns()

	select {
	case errOut := <-sub.Err():
		require.Error(t, errOut, "Err() must deliver a non-nil cause when transport dies")
	case <-time.After(2 * time.Second):
		t.Fatal("Err() did not surface inner subscription failure")
	}
}

// TestEthL1StateProvider_WatchStateUpdate_DecodeFailure verifies a malformed
// notification terminates the subscription with a decode error on Err() so
// l1.Client can react (a misformatted log means wrong contract or buggy node).
func TestEthL1StateProvider_WatchStateUpdate_DecodeFailure(t *testing.T) {
	const subID = "0xdec0de"
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

	s, err := l1.NewEthL1StateProvider(t.Context(), srv.WSURL(), eth.Address{})
	require.NoError(t, err)
	t.Cleanup(s.Close)

	sink := make(chan *l1.StateUpdate, 1)
	sub, err := s.WatchStateUpdate(t.Context(), sink)
	require.NoError(t, err)
	defer sub.Unsubscribe()

	// Correct topic, but a truncated data section - Decode rejects it.
	badLog := map[string]any{
		"topics":      []string{contract.LogStateUpdateSigHash.Hex()},
		"data":        "0x00",
		"blockNumber": "0x1",
		"removed":     false,
	}
	require.NoError(t, srv.PushNotification(t.Context(), subID, badLog))

	select {
	case errOut := <-sub.Err():
		require.Error(t, errOut)
		assert.Contains(t, errOut.Error(), "decoding")
	case <-time.After(2 * time.Second):
		t.Fatal("Err() did not fire on decode failure")
	}
}

// TestEthL1StateProvider_WatchStateUpdate_FailsAfterClose verifies that once the
// provider is closed, WatchStateUpdate refuses to dial rather than spin up a
// forwarder that never sees traffic.
func TestEthL1StateProvider_WatchStateUpdate_FailsAfterClose(t *testing.T) {
	srv := clienttest.NewTestServer(t)
	s, err := l1.NewEthL1StateProvider(t.Context(), srv.WSURL(), eth.Address{})
	require.NoError(t, err)

	s.Close()

	sink := make(chan *l1.StateUpdate, 1)
	_, err = s.WatchStateUpdate(t.Context(), sink)
	require.Error(t, err)
	assert.ErrorIs(t, err, l1.ErrClosed)
}

// TestEthL1StateProvider_WithLogger smoke-tests that a custom logger doesn't break
// construction or subsequent RPC calls.
func TestEthL1StateProvider_WithLogger(t *testing.T) {
	srv := clienttest.NewTestServer(t)
	srv.SetHandler(func(req clienttest.TestRequest) (any, *clienttest.TestRPCError) {
		if req.Method == "eth_chainId" {
			return "0x1", nil
		}
		return nil, &clienttest.TestRPCError{Code: -32601, Message: req.Method}
	})

	s, err := l1.NewEthL1StateProvider(t.Context(), srv.WSURL(), eth.Address{},
		l1.WithEthL1StateProviderLogger(log.NewNopZapLogger()),
	)
	require.NoError(t, err)
	t.Cleanup(s.Close)

	id, err := s.ChainID(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "1", id.String())
}

// TestEthL1StateProvider_DialError verifies the constructor wraps a dial failure.
// An unsupported URL scheme is the only way to force this without a real network.
func TestEthL1StateProvider_DialError(t *testing.T) {
	_, err := l1.NewEthL1StateProvider(t.Context(), "http://example.invalid", eth.Address{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dialing L1",
		"NewEthL1StateProvider must wrap the underlying dial error so the cause is identifiable")
}
