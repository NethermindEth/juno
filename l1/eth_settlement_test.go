package l1_test

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/l1"
	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/l1/eth/client"
	"github.com/NethermindEth/juno/l1/eth/contract"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingListener captures every OnL1Call invocation. Used to verify
// the deferred-observe contract (listener fires on both success AND
// failure paths so error rates are visible in metrics).
type recordingListener struct {
	mu    sync.Mutex
	calls []string // method names, in order
}

func (r *recordingListener) OnNewL1Head(_ *core.L1Head) {}
func (r *recordingListener) OnL1Call(method string, _ time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, method)
}

func (r *recordingListener) Methods() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.calls))
	copy(out, r.calls)
	return out
}

// TestEthSettlement_RedialsAfterTransportClosed verifies that when the
// underlying ws transport is killed (the failure mode that triggered
// the user-reported "subscribe to LogStateUpdate: subscribe to logs:
// transport closed" bug), the next RPC call auto-redials and succeeds.
//
// Without redial, the EthSettlement would be permanently stuck — every
// subsequent call returning ErrTransportClosed from the dead transport.
func TestEthSettlement_RedialsAfterTransportClosed(t *testing.T) {
	srv := client.NewTestServer(t)
	srv.SetHandler(func(req client.TestRequest) (any, *client.TestRPCError) {
		if req.Method == "eth_chainId" {
			return "0x539", nil
		}
		return nil, &client.TestRPCError{Code: -32601, Message: req.Method}
	})

	s, err := l1.NewEthSettlement(t.Context(), srv.WSURL(), eth.Address{})
	require.NoError(t, err)
	t.Cleanup(s.Close)

	// Sanity: first call works.
	id, err := s.ChainID(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "1337", id.String())

	// Kill the ws conn server-side; the client's readLoop sees the read
	// error and shuts the transport down asynchronously.
	srv.KillWSConns()

	// Give the client a moment to notice. Without this, the call below
	// could still hit the old (not-yet-shut-down) transport and hang on
	// the write, then time out instead of redialing.
	require.Eventually(t, func() bool {
		// Probe via ChainID; expect either ErrTransportClosed (transport
		// just shut down, redial hasn't happened yet) or success (redial
		// happened on this very call). Once the redialed path succeeds
		// we're done.
		_, err := s.ChainID(t.Context())
		return err == nil
	}, 2*time.Second, 20*time.Millisecond,
		"ChainID should succeed after transport drop via auto-redial")
}

// TestEthSettlement_AfterCloseReturnsErrClosed verifies Close is
// terminal: post-Close calls don't trigger another redial and instead
// return ErrSettlementClosed.
func TestEthSettlement_AfterCloseReturnsErrClosed(t *testing.T) {
	srv := client.NewTestServer(t)
	srv.SetHandler(func(req client.TestRequest) (any, *client.TestRPCError) {
		if req.Method == "eth_chainId" {
			return "0x1", nil
		}
		return nil, &client.TestRPCError{Code: -32601, Message: req.Method}
	})

	s, err := l1.NewEthSettlement(t.Context(), srv.WSURL(), eth.Address{})
	require.NoError(t, err)

	// First call succeeds.
	_, err = s.ChainID(t.Context())
	require.NoError(t, err)

	s.Close()

	_, err = s.ChainID(t.Context())
	require.Error(t, err)
	assert.ErrorIs(t, err, l1.ErrSettlementClosed)
}

// TestEthSettlement_ListenerFiresOnErrorPath verifies the
// deferred-observe contract: OnL1Call must fire on the failure path,
// not just success, so dashboards can distinguish "L1 healthy and
// quiet" from "L1 broken". Regression target — earlier code emitted
// the metric only on the success branch.
func TestEthSettlement_ListenerFiresOnErrorPath(t *testing.T) {
	srv := client.NewTestServer(t)
	// Every method errors out so we exercise the failure branch.
	srv.SetHandler(func(_ client.TestRequest) (any, *client.TestRPCError) {
		return nil, &client.TestRPCError{Code: -32000, Message: "boom"}
	})

	s, err := l1.NewEthSettlement(t.Context(), srv.WSURL(), eth.Address{})
	require.NoError(t, err)
	t.Cleanup(s.Close)

	rec := &recordingListener{}
	s.SetListener(rec)

	// All four methods should fail and STILL record an OnL1Call.
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

// TestEthSettlement_CloseIsIdempotent — double-Close shouldn't panic
// or block, and shouldn't trigger a redial.
func TestEthSettlement_CloseIsIdempotent(t *testing.T) {
	srv := client.NewTestServer(t)
	s, err := l1.NewEthSettlement(t.Context(), srv.WSURL(), eth.Address{})
	require.NoError(t, err)

	s.Close()
	s.Close()
}

// TestEthSettlement_PreservesErrNotFound verifies the sentinel
// wrapping survives the withRetryOnClosed helper — if a call returns
// eth.ErrNotFound (e.g. finalised block missing), callers can still
// errors.Is it. (Doesn't actually exercise a redial; that's covered by
// TestEthSettlement_RedialsAfterTransportClosed.)
func TestEthSettlement_PreservesErrNotFound(t *testing.T) {
	srv := client.NewTestServer(t)
	srv.SetHandler(func(req client.TestRequest) (any, *client.TestRPCError) {
		if req.Method == "eth_getBlockByNumber" {
			return nil, nil // null result → ErrNotFound at the client layer
		}
		return nil, &client.TestRPCError{Code: -32601, Message: req.Method}
	})

	s, err := l1.NewEthSettlement(t.Context(), srv.WSURL(), eth.Address{})
	require.NoError(t, err)
	t.Cleanup(s.Close)

	_, err = s.FinalisedHeight(t.Context())
	require.Error(t, err)
	assert.True(t, errors.Is(err, eth.ErrNotFound),
		"FinalisedHeight must wrap eth.ErrNotFound so callers can errors.Is it; got: %v", err)
}

// stateUpdateLogJSON returns a JSON-RPC-shaped LogStateUpdate event for
// the test server. The data section is the canonical 96-byte layout:
// globalRoot ‖ blockNumber (int256, low-8-bytes) ‖ blockHash. The
// returned map is consumed by encoding/json directly.
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

// TestEthSettlement_FilterStateUpdate_DecodesAndTranslates exercises the
// hot path through FilterStateUpdate: the eth_getLogs call lands raw
// logs, the contract decoder reads them, and stateUpdateFromContract
// reshapes them into chain-neutral StateUpdate. Without this test the
// translation layer is entirely unverified — both the decoder pipe and
// the field mapping (Removed, L1RefHeight, BlockNumber → L2BlockNumber).
func TestEthSettlement_FilterStateUpdate_DecodesAndTranslates(t *testing.T) {
	srv := client.NewTestServer(t)
	srv.SetHandler(func(req client.TestRequest) (any, *client.TestRPCError) {
		if req.Method == "eth_getLogs" {
			return []any{
				stateUpdateLogJSON(1, 1_000, false),
				stateUpdateLogJSON(2, 1_001, true),
			}, nil
		}
		return nil, &client.TestRPCError{Code: -32601, Message: req.Method}
	})

	s, err := l1.NewEthSettlement(t.Context(), srv.WSURL(), eth.Address{})
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

// TestEthSettlement_FilterStateUpdate_ErrorWrapsRange verifies the error
// message includes the [from, to] range so an operator chasing a
// "filter LogStateUpdate" failure can correlate it against the L1 sync
// loop's logs.
func TestEthSettlement_FilterStateUpdate_ErrorWrapsRange(t *testing.T) {
	srv := client.NewTestServer(t)
	srv.SetHandler(func(_ client.TestRequest) (any, *client.TestRPCError) {
		return nil, &client.TestRPCError{Code: -32000, Message: "query timeout"}
	})

	s, err := l1.NewEthSettlement(t.Context(), srv.WSURL(), eth.Address{})
	require.NoError(t, err)
	t.Cleanup(s.Close)

	_, err = s.FilterStateUpdate(t.Context(), 42, 99)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "[42,99]",
		"error must surface the requested range so operators can correlate")
	assert.Contains(t, err.Error(), "query timeout")
}

// TestEthSettlement_WatchStateUpdate_DeliversDecoded verifies the live
// subscribe path: eth_subscribe is issued, the server pushes a
// notification, the forwarder decodes via contract.Decode and emits a
// chain-neutral StateUpdate.
func TestEthSettlement_WatchStateUpdate_DeliversDecoded(t *testing.T) {
	const subID = "0xb10c"
	srv := client.NewTestServer(t)
	srv.SetHandler(func(req client.TestRequest) (any, *client.TestRPCError) {
		switch req.Method {
		case "eth_subscribe":
			return subID, nil
		case "eth_unsubscribe":
			return true, nil
		}
		return nil, &client.TestRPCError{Code: -32601, Message: req.Method}
	})

	s, err := l1.NewEthSettlement(t.Context(), srv.WSURL(), eth.Address{})
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

// TestEthSettlement_WatchStateUpdate_UnsubscribeIsClean verifies the
// forwarder's Unsubscribe path: Err() closes (no spurious error sent),
// and Unsubscribe is idempotent — a double call must not panic on the
// already-closed channels.
func TestEthSettlement_WatchStateUpdate_UnsubscribeIsClean(t *testing.T) {
	const subID = "0xbeef"
	srv := client.NewTestServer(t)
	srv.SetHandler(func(req client.TestRequest) (any, *client.TestRPCError) {
		switch req.Method {
		case "eth_subscribe":
			return subID, nil
		case "eth_unsubscribe":
			return true, nil
		}
		return nil, &client.TestRPCError{Code: -32601, Message: req.Method}
	})

	s, err := l1.NewEthSettlement(t.Context(), srv.WSURL(), eth.Address{})
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

// TestEthSettlement_WatchStateUpdate_PropagatesInnerErr verifies the
// forwarder.run() path that delivers a cause from the inner subscription
// onto its own Err() — the failure mode that the redial loop in l1.Client
// reacts to.
func TestEthSettlement_WatchStateUpdate_PropagatesInnerErr(t *testing.T) {
	const subID = "0xfa11"
	srv := client.NewTestServer(t)
	srv.SetHandler(func(req client.TestRequest) (any, *client.TestRPCError) {
		switch req.Method {
		case "eth_subscribe":
			return subID, nil
		case "eth_unsubscribe":
			return true, nil
		}
		return nil, &client.TestRPCError{Code: -32601, Message: req.Method}
	})

	s, err := l1.NewEthSettlement(t.Context(), srv.WSURL(), eth.Address{})
	require.NoError(t, err)
	t.Cleanup(s.Close)

	sink := make(chan *l1.StateUpdate, 1)
	sub, err := s.WatchStateUpdate(t.Context(), sink)
	require.NoError(t, err)
	defer sub.Unsubscribe()

	// Killing the conn server-side trips the inner subscription's
	// Err() → forwarder.run() picks it up → delivers it on the
	// caller-facing sub.Err().
	srv.KillWSConns()

	select {
	case errOut := <-sub.Err():
		require.Error(t, errOut, "Err() must deliver a non-nil cause when transport dies")
	case <-time.After(2 * time.Second):
		t.Fatal("Err() did not surface inner subscription failure")
	}
}

// TestEthSettlement_WatchStateUpdate_FailsAfterClose verifies the
// Close-then-Watch ordering: once the settlement is terminally closed,
// WatchStateUpdate refuses to dial a new subscription instead of
// silently spinning up a forwarder that will never see traffic.
func TestEthSettlement_WatchStateUpdate_FailsAfterClose(t *testing.T) {
	srv := client.NewTestServer(t)
	s, err := l1.NewEthSettlement(t.Context(), srv.WSURL(), eth.Address{})
	require.NoError(t, err)

	s.Close()

	sink := make(chan *l1.StateUpdate, 1)
	_, err = s.WatchStateUpdate(t.Context(), sink)
	require.Error(t, err)
	assert.ErrorIs(t, err, l1.ErrSettlementClosed)
}

// TestEthSettlement_WithSettlementLogger smoke-tests the option wiring
// — supplying a custom logger to NewEthSettlement must not break
// construction or subsequent RPC calls. The internal forwarding to the
// underlying client is covered indirectly by every other test, which
// only differs from this one in the absence of the option.
func TestEthSettlement_WithSettlementLogger(t *testing.T) {
	srv := client.NewTestServer(t)
	srv.SetHandler(func(req client.TestRequest) (any, *client.TestRPCError) {
		if req.Method == "eth_chainId" {
			return "0x1", nil
		}
		return nil, &client.TestRPCError{Code: -32601, Message: req.Method}
	})

	s, err := l1.NewEthSettlement(t.Context(), srv.WSURL(), eth.Address{},
		l1.WithSettlementLogger(log.NewNopZapLogger()),
	)
	require.NoError(t, err)
	t.Cleanup(s.Close)

	id, err := s.ChainID(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "1", id.String())
}

// TestEthSettlement_NewEthSettlement_DialError verifies the constructor
// fails cleanly when the underlying client.New call cannot dial. The
// only way to force this without a real network is an unsupported URL
// scheme, which client.New rejects in url-parse before it hits the wire.
func TestEthSettlement_NewEthSettlement_DialError(t *testing.T) {
	_, err := l1.NewEthSettlement(t.Context(), "http://example.invalid", eth.Address{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dial L1",
		"NewEthSettlement must wrap the underlying dial error so the cause is identifiable")
}
