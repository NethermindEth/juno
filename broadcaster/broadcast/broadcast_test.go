package broadcast_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NethermindEth/juno/broadcaster/broadcast"
	"github.com/NethermindEth/juno/broadcaster/broadcast/ring"
	"github.com/stretchr/testify/require"
)

// recvWithTimeout receives with a timeout to avoid hanging tests.
func recvWithTimeout[T any](t *testing.T, ch <-chan T, d time.Duration) (T, bool) {
	t.Helper()
	var zero T
	select {
	case v, ok := <-ch:
		return v, ok
	case <-time.After(d):
		return zero, false
	}
}

// subscribe creates a subscription and returns its receive channel plus an
// unsubscribe function. The concrete subscription type is unexported, so this
// helper keeps callers from having to name it.
func subscribe[T any](t *testing.T, b *broadcast.Broadcast[T]) (<-chan ring.EventOrLag[T], func()) {
	t.Helper()
	sub := b.NewSubscribable().Subscribe()
	return sub.Recv(), sub.Unsubscribe
}

// startupDelay gives a freshly created subscription's iterator goroutine time to
// capture its starting sequence (tail+1) before the first Send. Unlike the old
// synchronous Subscribe, the new iterator pins its start point asynchronously.
const startupDelay = 20 * time.Millisecond

func TestBasicSendRecvNoLag(t *testing.T) {
	numEvents := 100

	bcast := broadcast.New[int](uint64(numEvents))
	pub := bcast.NewPublisher()

	recv, unsub := subscribe(t, bcast)
	defer unsub()
	time.Sleep(startupDelay)

	// produce
	go func() {
		for i := range numEvents {
			pub.Send(i)
		}
	}()

	// consume
	for i := range numEvents {
		ev, ok := recvWithTimeout(t, recv, 3*time.Second)
		require.True(t, ok, "out channel closed prematurely at i=%d", i)
		require.True(t, ev.IsEvent())
		val, ok := ev.AsEvent()
		require.True(t, ok)
		require.Equal(t, i, val) // strictly increasing from 0..numEvents-1
	}
}

func TestMultipleSubscribersReceiveSameData(t *testing.T) {
	numEvents := 1000
	numSubscribers := 100
	bcast := broadcast.New[int](uint64(numEvents))
	pub := bcast.NewPublisher()

	// start consumers
	var wg sync.WaitGroup
	wg.Add(numSubscribers)
	for range numSubscribers {
		recv, unsub := subscribe(t, bcast)
		go func() {
			defer wg.Done()
			defer unsub()
			for i := range numEvents {
				ev, ok := recvWithTimeout(t, recv, time.Second)
				require.True(t, ok, "subscriber out closed early")
				require.True(t, ev.IsEvent())
				val, ok := ev.AsEvent()
				require.True(t, ok)
				require.Equal(t, i, val)
			}
		}()
	}

	time.Sleep(startupDelay)
	// publish
	for i := range numEvents {
		pub.Send(i)
	}

	wg.Wait()
}

func TestConcurrentProducers_AllDelivered_NoLag(t *testing.T) {
	type payload struct {
		Prod int
		Seq  int
	}
	M := 8                    // producers
	K := 500                  // messages per producer
	capacity := uint64(M * K) // ensure no overwrite to keep test simple
	bc := broadcast.New[payload](capacity)

	recv, unsub := subscribe(t, bc)
	defer unsub()
	time.Sleep(startupDelay)

	pubs := make([]broadcast.Publisher[payload], M)
	for p := range M {
		pubs[p] = bc.NewPublisher()
	}

	// launch producers
	var wg sync.WaitGroup
	for p := range M {
		pid, pub := p, pubs[p]
		wg.Go(func() {
			for i := range K {
				pub.Send(payload{Prod: pid, Seq: i})
			}
		})
	}
	wg.Wait()

	// Consume all M*K messages and verify per-producer ordering is non-decreasing
	seen := make([]int, M)
	for i := range M {
		seen[i] = -1
	}

	total := M * K
	for n := range total {
		ev, ok := recvWithTimeout(t, recv, time.Second)
		require.True(t, ok, "out closed early at n=%d", n)
		require.True(t, ev.IsEvent(), "did not expect lag in this test")
		p, ok := ev.AsEvent()
		require.True(t, ok)
		require.Equal(t, seen[p.Prod]+1, p.Seq, "per-producer order must be preserved")
		seen[p.Prod] = p.Seq
	}
}

func TestSlowConsumerDoesNotDeadlock(t *testing.T) {
	// One slow subscriber; producer sends bursts that overrun the delivery buffer.
	bufferSize := uint64(64)
	bc := broadcast.New[int](bufferSize)
	pub := bc.NewPublisher()

	recv, unsub := subscribe(t, bc)
	defer unsub()

	// Slow consumer
	var recvCount uint64
	var wg sync.WaitGroup
	wg.Go(func() {
		timeout := time.After(time.Second)
		for {
			select {
			case ev, open := <-recv:
				if !open {
					return
				}
				// simulate slow processing
				time.Sleep(2 * time.Millisecond)

				// verify item type is plausible (either event or lag)
				if ev.IsEvent() {
					_, _ = ev.AsEvent()
				} else {
					_, _ = ev.AsLag()
				}
				recvCount++
				if recvCount >= 300 {
					return
				}
			case <-timeout:
				return
			}
		}
	})

	sent := 0
	// Producer sends bursts
	for b := range 30 {
		for i := range 20 {
			msg := b*1000 + i
			pub.Send(msg)
			sent++
		}
		time.Sleep(1 * time.Millisecond) // small pause between bursts
	}

	wg.Wait()
	require.GreaterOrEqual(
		t,
		recvCount,
		bufferSize,
		"subscriber must make progress despite slow processing",
	)
}

func TestLaggedDetection(t *testing.T) {
	bc := broadcast.New[int](2)
	pub := bc.NewPublisher()

	recv, unsub := subscribe(t, bc)
	defer unsub()
	time.Sleep(startupDelay)

	for i := range 2 {
		msg := i + 1
		pub.Send(msg)
	}

	// Wait for the subscriber to deliver seq 1 to the channel and pull seq 2 into
	// memory so the following overwrite is observed as a lag.
	time.Sleep(5 * time.Millisecond)
	pub.Send(3)
	pub.Send(4)
	pub.Send(5) // Lag

	// move cursor to missing seq 3
	for i := range 2 {
		ev := <-recv
		require.True(t, ev.IsEvent())
		event, ok := ev.AsEvent()
		require.True(t, ok)
		require.Equal(t, i+1, event)
	}

	// Subscriber's cursor is at the third message, which has been overwritten.
	lagEv := <-recv
	lag, ok := lagEv.AsLag()
	require.True(t, ok)
	require.Equal(t, ring.LaggedError{
		MissedSeq: 3,
		NextSeq:   4,
	}, lag)
}

func TestLagResyncAfterError(t *testing.T) {
	bc := broadcast.New[int](2)
	pub := bc.NewPublisher()

	recv, unsub := subscribe(t, bc)
	defer unsub()
	time.Sleep(startupDelay)

	for i := range 2 {
		msg := i + 1
		pub.Send(msg)
	}

	time.Sleep(5 * time.Millisecond)
	pub.Send(3)
	pub.Send(4)
	pub.Send(5) // Lag

	for i := range 2 {
		ev := <-recv
		require.True(t, ev.IsEvent())
		event, ok := ev.AsEvent()
		require.True(t, ok)
		require.Equal(t, i+1, event)
	}

	lagEv := <-recv
	lag, ok := lagEv.AsLag()
	require.True(t, ok)
	require.Equal(
		t,
		ring.LaggedError{
			MissedSeq: 3,
			NextSeq:   4,
		},
		lag,
	)

	// Now that the cursor has advanced to the oldest available, next should be 4.
	nextEv := <-recv
	event, ok := nextEv.AsEvent()
	require.True(t, ok)
	require.Equal(t, 4, event)
}

func TestUnsubscribe_ClosesOut(t *testing.T) {
	bc := broadcast.New[int](2)

	recv, unsub := subscribe(t, bc)
	unsub()

	_, open := recvWithTimeout(t, recv, 500*time.Millisecond)
	require.False(t, open, "out must be closed after Unsubscribe")
}

func TestUnsubscribeMidRingExitsPromptly(t *testing.T) {
	// Unsubscribing while the subscriber is still working through a full ring must
	// close its channel promptly. The iterator does not drain the ring on cancel, so
	// only assert that whatever it delivered is an in-order prefix.
	numEvents := uint64(64)
	bc := broadcast.New[uint64](numEvents)
	pub := bc.NewPublisher()

	recv, unsub := subscribe(t, bc)
	time.Sleep(startupDelay)

	for i := range numEvents {
		pub.Send(i)
	}

	time.Sleep(50 * time.Millisecond)
	unsub()

	var received uint64
	closed := false
	deadline := time.After(2 * time.Second)
loop:
	for {
		select {
		case ev, open := <-recv:
			if !open {
				closed = true
				break loop
			}
			require.True(t, ev.IsEvent())
			event, ok := ev.AsEvent()
			require.True(t, ok)
			require.Equal(t, received, event, "delivered events must be an in-order prefix")
			received++
		case <-deadline:
			break loop
		}
	}
	require.True(t, closed, "out should close promptly after Unsubscribe")
}

func TestSubscribeUnsubscribeDuringHotSend(t *testing.T) {
	bc := broadcast.New[int](64)
	pub := bc.NewPublisher()

	// Hot publisher, stopped via ctx and joined before the test returns.
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var pubWg sync.WaitGroup
	pubWg.Go(func() {
		i := 0
		for {
			select {
			case <-ctx.Done():
				return
			default:
				pub.Send(i)
				i++
			}
		}
	})

	// Rapid subscribe/unsubscribe
	var wg sync.WaitGroup
	workers := 16
	for range workers {
		wg.Go(func() {
			for range 50 {
				recv, unsub := subscribe(t, bc)
				// read a few items and unsubscribe
				read := 0
				for read < 3 {
					ev, ok := recvWithTimeout(t, recv, time.Second)
					require.True(t, ok, "out closed unexpectedly during hot send")
					if ev.IsEvent() || ev.IsLag() {
						read++
					}
				}
				unsub()
			}
		})
	}

	wg.Wait()
	cancel()
	pubWg.Wait()
}

func TestUnsubscribeDuringHotSend(t *testing.T) {
	bc := broadcast.New[int](1024)

	// start publishers
	var pubWg sync.WaitGroup
	producers := 4
	stop := atomic.Bool{}
	for range producers {
		pub := bc.NewPublisher()
		pubWg.Go(func() {
			i := 0
			for !stop.Load() {
				pub.Send(i)
				i++
			}
		})
	}
	defer func() {
		stop.Store(true)
		pubWg.Wait()
	}()

	// start subscribers, each draining until its channel closes
	var subWg sync.WaitGroup
	unsubs := make([]func(), 0, 8)
	for range 8 {
		recv, unsub := subscribe(t, bc)
		unsubs = append(unsubs, unsub)
		subWg.Go(func() {
			for range recv {
			}
		})
	}

	// Unsubscribe every subscriber while producers are still hot.
	time.AfterFunc(100*time.Millisecond, func() {
		for _, unsub := range unsubs {
			unsub()
		}
	})

	done := make(chan struct{})
	go func() {
		subWg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "timeout waiting for subscribers to exit after Unsubscribe")
	}
}
