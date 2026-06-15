package l1_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/l1"
	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/l1/eth/client"
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
		switch req.Method {
		case "eth_chainId":
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
