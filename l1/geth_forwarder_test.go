package l1

import (
	"errors"
	"math/big"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/l1/geth/contract"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeEventSub is a minimal event.Subscription that lets the test drive
// error delivery and observe Unsubscribe.
type fakeEventSub struct {
	errCh    chan error
	unsubbed atomic.Bool
}

func newFakeEventSub() *fakeEventSub {
	return &fakeEventSub{errCh: make(chan error, 1)}
}

func (f *fakeEventSub) Err() <-chan error { return f.errCh }

func (f *fakeEventSub) Unsubscribe() {
	if f.unsubbed.CompareAndSwap(false, true) {
		close(f.errCh)
	}
}

// newForwarderForTest wires forwardStateUpdates to a fake inner subscription
// and returns the resulting eth.Subscription along with the writable raw
// channel the test feeds contract events into.
func newForwarderForTest(
	sink chan *StateUpdate, inner *fakeEventSub,
) (eth.Subscription, chan *contract.StarknetLogStateUpdate) {
	raw := make(chan *contract.StarknetLogStateUpdate, 1)
	sub := forwardStateUpdates(inner, raw, sink)
	return sub, raw
}

func sampleStarknetLogStateUpdate() *contract.StarknetLogStateUpdate {
	return &contract.StarknetLogStateUpdate{
		BlockNumber: big.NewInt(42),
		BlockHash:   big.NewInt(0xabc),
		GlobalRoot:  big.NewInt(0xdef),
		Raw: types.Log{
			BlockNumber: 99,
			Removed:     false,
		},
	}
}

// expectErrChClosed asserts that Err() is closed (drained of any pending
// value, then EOF) within the deadline. event.NewSubscription closes the
// error channel exactly once after the producer returns, so a closed Err()
// proves the forwarding goroutine has torn down.
func expectErrChClosed(t *testing.T, ch <-chan error) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for Err() to close")
		}
	}
}

func TestForwardStateUpdates_NormalForward(t *testing.T) {
	sink := make(chan *StateUpdate, 1)
	inner := newFakeEventSub()
	sub, raw := newForwarderForTest(sink, inner)
	t.Cleanup(sub.Unsubscribe)

	ev := sampleStarknetLogStateUpdate()
	raw <- ev

	select {
	case got := <-sink:
		require.NotNil(t, got)
		assert.Equal(t, uint64(42), got.L2BlockNumber)
		assert.Equal(t, uint64(99), got.L1RefHeight)
		assert.False(t, got.Removed)
		assert.Equal(t, new(felt.Felt).SetBigInt(ev.BlockHash), got.L2BlockHash)
		assert.Equal(t, new(felt.Felt).SetBigInt(ev.GlobalRoot), got.StateRoot)
	case <-time.After(2 * time.Second):
		t.Fatal("event not forwarded to sink")
	}

	// Err() should still be open while the forwarder is running.
	select {
	case _, ok := <-sub.Err():
		t.Fatalf("Err() unexpectedly closed/delivered while running (ok=%v)", ok)
	default:
	}
}

func TestForwardStateUpdates_InnerErrorDeliversAndCloses(t *testing.T) {
	sink := make(chan *StateUpdate, 1)
	inner := newFakeEventSub()
	sub, _ := newForwarderForTest(sink, inner)

	cause := errors.New("upstream gone")
	inner.errCh <- cause

	select {
	case err := <-sub.Err():
		require.ErrorIs(t, err, cause)
	case <-time.After(2 * time.Second):
		t.Fatal("Err() did not deliver the cause")
	}
	// After the cause is delivered the channel must close — a second
	// receive yields (zero, !ok).
	select {
	case err, ok := <-sub.Err():
		assert.False(t, ok, "Err() should be closed after delivering cause")
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Err() not closed after delivering cause")
	}
}

func TestForwardStateUpdates_UnsubscribeExitsGoroutine(t *testing.T) {
	sink := make(chan *StateUpdate, 1)
	inner := newFakeEventSub()
	sub, _ := newForwarderForTest(sink, inner)

	sub.Unsubscribe()
	assert.True(t, inner.unsubbed.Load(), "inner subscription should be Unsubscribed")
	expectErrChClosed(t, sub.Err())
}

func TestForwardStateUpdates_SinkStalledThenUnsubscribe(t *testing.T) {
	// Unbuffered sink, never drained: if the forwarding loop reaches the
	// `case sink <- ...` branch it will block there. Unsubscribe() must
	// unblock it via the <-quit arm of the inner select.
	sink := make(chan *StateUpdate)
	inner := newFakeEventSub()
	sub, raw := newForwarderForTest(sink, inner)

	raw <- sampleStarknetLogStateUpdate()
	sub.Unsubscribe()

	expectErrChClosed(t, sub.Err())
	assert.True(t, inner.unsubbed.Load())
}
