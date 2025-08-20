package broadcast_test

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NethermindEth/juno/broadcast"
	"github.com/stretchr/testify/require"
)

// helper: receive with timeout to avoid hanging tests
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

func TestBasicSendRecvNoLag(t *testing.T) {
	numEvents := 100
	bcast := broadcast.New[int](uint64(numEvents))
	defer bcast.Close()

	sub := bcast.Subscribe()
	defer sub.Unsubscribe()

	// produce
	go func() {
		for i := range numEvents {
			require.NoError(t, bcast.Send(i))
		}
	}()

	// consume
	for i := range numEvents {
		ev, ok := recvWithTimeout(t, sub.Recv(), 3*time.Second)
		require.True(t, ok, "out channel closed prematurely at i=%d", i)
		require.True(t, ev.IsEvent())
		val, err := ev.Event()
		require.NoError(t, err)
		require.Equal(t, i, val) // strictly increasing from 0..numEvents-1
	}
}

func TestMultipleSubscribersReceiveSameData(t *testing.T) {
	numEvents := 1000
	numSubscribers := 100
	bcast := broadcast.New[int](uint64(numEvents))
	defer bcast.Close()

	// start consumers
	var wg sync.WaitGroup
	wg.Add(numSubscribers)
	for range numSubscribers {
		sub := bcast.Subscribe()
		go func(sub broadcast.Subscription[int]) {
			defer wg.Done()
			defer sub.Unsubscribe()
			for i := range numEvents {
				ev, ok := recvWithTimeout(t, sub.Recv(), time.Second)
				require.True(t, ok, "subscriber out closed early")
				require.True(t, ev.IsEvent())
				val, err := ev.Event()
				require.NoError(t, err)
				require.Equal(t, i, val)
			}
		}(sub)
	}

	// publish
	for i := range numEvents {
		require.NoError(t, bcast.Send(i))
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
	defer bc.Close()

	sub := bc.Subscribe()
	defer sub.Unsubscribe()

	// launch producers
	var wg sync.WaitGroup
	wg.Add(M)
	for p := range M {
		pid := p
		go func() {
			defer wg.Done()
			for i := range K {
				require.NoError(t, bc.Send(payload{Prod: pid, Seq: i}))
			}
		}()
	}
	wg.Wait()

	// Consume all M*K messages and verify per-producer ordering is non-decreasing
	seen := make([]int, M)
	for i := range M {
		seen[i] = -1
	}

	total := M * K
	for n := range total {
		ev, ok := recvWithTimeout(t, sub.Recv(), time.Second)
		require.True(t, ok, "out closed early at n=%d", n)
		require.True(t, ev.IsEvent(), "did not expect lag in this test")
		p, err := ev.Event()
		require.NoError(t, err)
		require.Equal(t, seen[p.Prod]+1, p.Seq, "per-producer order must be preserved")
		seen[p.Prod] = p.Seq
	}
}

func TestNotifyCoalescingDoesNotDeadlock(t *testing.T) {
	// One slow subscriber; producer sends bursts that overrun notifyC (buffer=1).
	bufferSize := uint64(64)
	bc := broadcast.New[int](bufferSize)

	sub := bc.Subscribe()
	defer sub.Unsubscribe()

	// Slow consumer
	var recvCount uint64
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		timeout := time.After(time.Second)
		for {
			select {
			case ev, open := <-sub.Recv():
				if !open {
					return
				}
				// simulate slow processing
				time.Sleep(2 * time.Millisecond)

				// verify item type is plausible (either event or lag)
				if ev.IsEvent() {
					_, _ = ev.Event()
				} else {
					_, _ = ev.Lag()
				}
				recvCount++
				if recvCount >= 300 {
					return
				}
			case <-timeout:
				return
			}
		}
	}()

	sent := 0
	// Producer sends bursts
	for b := range 30 {
		for i := range 20 {
			require.NoError(t, bc.Send(b*1000+i))
			sent++
		}
		time.Sleep(1 * time.Millisecond) // small pause between bursts
	}
	time.Sleep(1 * time.Second)
	bc.Close()

	wg.Wait()
	require.GreaterOrEqual(
		t,
		recvCount,
		bufferSize,
		"subscriber must make progress despite coalesced notifies",
	)
}

func TestLaggedDetection(t *testing.T) {
	bc := broadcast.New[int](2)
	sub := bc.Subscribe()
	require.NoError(t, bc.Send(1))
	require.NoError(t, bc.Send(2))
	// wait for subscriber to take first value to chan and
	// second value to memory to avoid race and consistency of test
	time.Sleep(5 * time.Millisecond)
	require.NoError(t, bc.Send(3))
	require.NoError(t, bc.Send(4))
	require.NoError(t, bc.Send(5)) // Lag

	// move cursor to missing seq 3
	for i := range 2 {
		ev := <-sub.Recv()
		require.True(t, ev.IsEvent())
		event, err := ev.Event()
		require.NoError(t, err)
		require.Equal(t, i+1, event)
	}

	// Subscriber's cursor is at third message
	// which should be gone
	lagEv := <-sub.Recv()
	lag, err := lagEv.Lag()
	require.NoError(t, err)
	require.Equal(t, broadcast.LaggedInfo{
		MissedSeq: 3,
		NextSeq:   4,
	}, lag)
}

func TestLagResyncAfterError(t *testing.T) {
	bc := broadcast.New[int](2)
	sub := bc.Subscribe()
	require.NoError(t, bc.Send(1)) // in out chan
	require.NoError(t, bc.Send(2))
	// Sleep to allow forwarder goroutine to put 1 to chan and 2 to memory
	time.Sleep(5 * time.Millisecond)
	require.NoError(t, bc.Send(3))
	require.NoError(t, bc.Send(4))
	require.NoError(t, bc.Send(5)) // lag

	for i := range 2 {
		ev := <-sub.Recv()
		require.True(t, ev.IsEvent())
		event, err := ev.Event()
		require.NoError(t, err)
		require.Equal(t, i+1, event)
	}

	lagEv := <-sub.Recv()
	lag, err := lagEv.Lag()
	require.NoError(t, err)
	require.Equal(
		t,
		broadcast.LaggedInfo{
			MissedSeq: 3,
			NextSeq:   4,
		},
		lag,
	)

	// Now that cursor has advanced, next should be 4
	nextEv := <-sub.Recv()
	event, err := nextEv.Event()
	require.NoError(t, err)
	require.Equal(t, 4, event)
}

func TestCloseBroadcast(t *testing.T) {
	bc := broadcast.New[int](2)
	sub := bc.Subscribe()
	require.NoError(t, bc.Send(1))
	time.Sleep(5 * time.Millisecond)
	bc.Close()
	out := sub.Recv()
	// receive one
	val, open := <-out
	require.True(t, open)
	event, err := val.Event()
	require.NoError(t, err)
	require.Equal(t, 1, event)
	_, open = <-out
	require.False(t, open)
	require.ErrorIs(t, bc.Send(42), broadcast.ErrClosed)
}

func TestUnsubscribe_ClosesOut(t *testing.T) {
	bc := broadcast.New[int](2)
	sub := bc.Subscribe()
	sub.Unsubscribe()

	_, open := recvWithTimeout(t, sub.Recv(), 500*time.Millisecond)
	require.False(t, open, "out must be closed after Unsubscribe")
}

func TestErrClosedHandled_RunExitsAfterDrain(t *testing.T) {
	// This test expects the subscriber goroutine to exit promptly after Close.
	// If the implementation spins on ErrClosed in sub.run(), this test will fail (timeout).
	numEvents := uint64(64)
	bc := broadcast.New[uint64](numEvents)
	sub := bc.Subscribe()

	for i := range numEvents {
		require.NoError(t, bc.Send(i))
	}

	// Close immediately, then ensure Out closes soon
	bc.Close()

	// Drain the buffer
	for i := range numEvents {
		ev, ok := recvWithTimeout(t, sub.Recv(), time.Second)
		require.True(t, ok)
		require.True(t, ev.IsEvent())
		event, err := ev.Event()
		require.NoError(t, err)
		require.Equal(t, i, event)
	}
	_, ok := recvWithTimeout(t, sub.Recv(), time.Second)
	require.False(t, ok, "out should be closed after drain")
}

func TestUnsubscribe(t *testing.T) {
	bc := broadcast.New[int](2)
	sub := bc.Subscribe()
	sub.Unsubscribe()
	_, open := <-sub.Recv()
	require.False(t, open)
}

func TestUnsubscribe_Idempotent(t *testing.T) {
	bc := broadcast.New[int](2)
	sub := bc.Subscribe()
	sub.Unsubscribe()
	sub.Unsubscribe() // should not panic
}

func TestSubscribeUnsubscribeDuringHotSend(t *testing.T) {
	bc := broadcast.New[int](64)
	defer bc.Close()

	// Hot publisher
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go func() {
		i := 0
		for {
			select {
			case <-ctx.Done():
				return
			default:
				_ = bc.Send(i) // ignore ErrClosed on shutdown
				i++
			}
		}
	}()

	// Rapid subscribe/unsubscribe
	var wg sync.WaitGroup
	workers := 16
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			for range 50 {
				sub := bc.Subscribe()
				// read a few items and unsubscribe
				read := 0
				for read < 3 {
					ev, ok := recvWithTimeout(t, sub.Recv(), time.Second)
					require.True(t, ok, "out closed unexpectedly during hot send")
					if ev.IsEvent() || ev.IsLag() {
						read++
					}
				}
				sub.Unsubscribe()
			}
		}()
	}

	wg.Wait()
	cancel()
}

func TestCloseDuringHotSendAndSubscribe(t *testing.T) {
	bc := broadcast.New[int](1024)

	// start publishers
	var pubWg sync.WaitGroup
	producers := 4
	stop := atomic.Bool{}
	for range producers {
		pubWg.Add(1)
		go func() {
			defer pubWg.Done()
			i := 0
			for !stop.Load() {
				_ = bc.Send(i) // will return ErrClosed after Close; that's fine to ignore here
				i++
			}
		}()
	}

	// start subscribers
	var subWg sync.WaitGroup
	for range 8 {
		sub := bc.Subscribe()
		subWg.Add(1)
		go func(s broadcast.Subscription[int]) {
			defer subWg.Done()
			for range s.Recv() {
				// drain until closed
			}
		}(sub)
	}

	// Close during activity
	time.AfterFunc(100*time.Millisecond, func() {
		stop.Store(true)
		bc.Close()
	})

	// Ensure subscribers close within timeout
	done := make(chan struct{})
	go func() {
		pubWg.Wait()
		// After close Recv chan should be closed soon after drain
		subWg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "timeout waiting for shutdown after Close")
	}
}

func BenchmarkBroadcastReceiverDrain_PreFill(b *testing.B) {
	numSubscribers := []int{1, 4, 32, 128, 256, 512, 1024}
	numMessages := []int{1024, 2048, 4096}

	for _, nSubs := range numSubscribers {
		for _, numMsg := range numMessages {
			b.Run("subs="+strconv.Itoa(nSubs)+", msg_count="+strconv.Itoa(numMsg), func(b *testing.B) {
				bc := broadcast.New[int](uint64(numMsg))

				subs := make([]broadcast.Subscription[int], nSubs)
				for i := range subs {
					subs[i] = bc.Subscribe()
				}

				for i := range numMsg {
					require.NoError(b, bc.Send(i))
				}
				// Close broadcast. Drain-on-close semantics.
				bc.Close()
				var wg sync.WaitGroup
				wg.Add(len(subs))

				b.ResetTimer()
				// All subscribers start draining at once
				for _, sub := range subs {
					go func(sub broadcast.Subscription[int]) {
						defer wg.Done()
						ctr := 0
						out := sub.Recv()
						for {
							_, ok := <-out
							if !ok {
								return
							}
							ctr++

							if ctr == numMsg {
								return
							}
						}
					}(sub)
				}

				wg.Wait()
				b.StopTimer()
				for _, sub := range subs {
					sub.Unsubscribe()
				}
			})
		}
	}
}

func BenchmarkSubsribe(b *testing.B) {
	numSubscribers := []int{1, 4, 32, 128, 256, 512, 1024}
	for _, nSubs := range numSubscribers {
		b.Run("subs="+strconv.Itoa(nSubs), func(b *testing.B) {
			bc := broadcast.New[int](1024)

			subs := make([]broadcast.Subscription[int], nSubs)

			for b.Loop() {
				for i := range nSubs {
					subs[i] = bc.Subscribe()
				}
			}

			for _, sub := range subs {
				sub.Unsubscribe()
			}
		})
	}
}

func BenchmarkBroadcastSenderThroughput_Small(b *testing.B) {
	bc := broadcast.New[int](1024)

	const nSubs = 32
	subs := make([]broadcast.Subscription[int], nSubs)
	for i := range nSubs {
		subs[i] = bc.Subscribe()
	}

	var wg sync.WaitGroup
	wg.Add(nSubs)
	for _, sub := range subs {
		go func(s broadcast.Subscription[int]) {
			defer wg.Done()
			for range s.Recv() {
				// drain
			}
		}(sub)
	}

	b.ResetTimer()
	for i := range b.N {
		if err := bc.Send(i); err != nil {
			if err == broadcast.ErrClosed {
				break
			}
			b.Fatal(err)
		}
	}
	b.StopTimer()

	bc.Close()
	wg.Wait()
}

func benchmarkBroadcastSenderThroughput[T any](
	b *testing.B,
	newBroadcast func(uint64) broadcast.Broadcast[T],
	makePayload func(i int) T,
	label string,
	bufferSizes []uint64,
) {
	for _, bufSize := range bufferSizes {
		b.Run(fmt.Sprintf("%s_buffer=%d", label, bufSize), func(b *testing.B) {
			numSubscribers := []int{1, 4, 32, 128, 256, 512, 1024}
			for _, nSubs := range numSubscribers {
				b.Run(fmt.Sprintf("subs=%d", nSubs), func(b *testing.B) {
					bc := newBroadcast(bufSize)

					subs := make([]broadcast.Subscription[T], nSubs)
					for i := range subs {
						subs[i] = bc.Subscribe()
					}

					var wg sync.WaitGroup
					wg.Add(len(subs))

					type counts struct {
						recvd uint64
						lag   uint64
					}
					countCh := make(chan counts, nSubs)

					for _, sub := range subs {
						go func(sub broadcast.Subscription[T]) {
							defer wg.Done()
							defer sub.Unsubscribe()

							recvd := uint64(0)
							lag := uint64(0)
							out := sub.Recv()
							for ev := range out {
								if ev.IsEvent() {
									recvd++
								} else if ev.IsLag() {
									lag++
								}
							}

							countCh <- counts{recvd, lag}
						}(sub)
					}

					b.ResetTimer()
					start := time.Now()
					for i := range b.N {
						payload := makePayload(i)
						if err := bc.Send(payload); err != nil {
							b.Fatal(err)
						}
					}
					b.StopTimer()

					bc.Close()
					wg.Wait()
					close(countCh)

					duration := time.Since(start).Seconds()

					var totalRecvd uint64
					var totalLag uint64
					minRecvd := ^uint64(0) // max uint64
					maxRecvd := uint64(0)

					for c := range countCh {
						totalRecvd += c.recvd
						totalLag += c.lag

						minRecvd = min(minRecvd, c.recvd)
						maxRecvd = min(maxRecvd, c.recvd)
					}

					avgRecvd := float64(totalRecvd) / float64(nSubs)
					avgLag := float64(totalLag) / float64(nSubs)

					b.ReportMetric(float64(b.N)/duration, "msgs_sent_per_sec")
					b.ReportMetric(float64(totalRecvd)/duration, "msgs_recv_per_sec")
					b.ReportMetric(avgRecvd, "avg_msgs_recv_per_sub")
					b.ReportMetric(avgLag, "avg_lag_per_sub")
					b.ReportMetric(float64(totalLag)/float64(b.N), "lag_per_msg")

					b.Logf("%s buffer=%d subs=%d Sent: %d, Received (total): %d, Lag (total): %d, Duration: %.3fs",
						label, bufSize, nSubs, b.N, totalRecvd, totalLag, duration)
					b.Logf("%s buffer=%d subs=%d Received min/max per subscriber: %d / %d",
						label, bufSize, nSubs, minRecvd, maxRecvd)
					b.Logf("%s buffer=%d subs=%d Received avg per subscriber: %.2f, Lag avg per subscriber: %.2f",
						label, bufSize, nSubs, avgRecvd, avgLag)
				})
			}
		})
	}
}

func BenchmarkBroadcastPublisherThroughput(b *testing.B) {
	bufferSizes := []uint64{64, 256, 512, 1024, 2048, 4096}

	// Run benchmark with int values as payload
	benchmarkBroadcastSenderThroughput(b,
		func(capacity uint64) broadcast.Broadcast[int] {
			return broadcast.New[int](capacity)
		},
		func(i int) int {
			return i
		},
		"int_value",
		bufferSizes,
	)

	// Run benchmark with pointers to int as payload
	benchmarkBroadcastSenderThroughput(b,
		func(capacity uint64) broadcast.Broadcast[*int] {
			return broadcast.New[*int](capacity)
		},
		func(i int) *int {
			v := i
			return &v
		},
		"int_ptr",
		bufferSizes,
	)
}
