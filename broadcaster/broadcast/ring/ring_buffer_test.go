package ring_test

import (
	"context"
	"iter"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/NethermindEth/juno/broadcaster/broadcast/ring"
	"github.com/stretchr/testify/require"
)

func TestNewRingBuffer(t *testing.T) {
	type testCase struct {
		description      string
		input            uint64
		expectedCapacity uint64
	}

	testCases := []testCase{
		{description: "capacity 5 should round to 8", input: 5, expectedCapacity: 8},
		{description: "capacity 3 should round to 4", input: 3, expectedCapacity: 4},
		{description: "capacity 16 should stay 16", input: 16, expectedCapacity: 16},
		{description: "capacity 0 should round to 1", input: 0, expectedCapacity: 1},
	}

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			rb := ring.NewRingBuffer[int](testCase.input)
			require.Equal(t, testCase.expectedCapacity, rb.Capacity(),
				"capacity should be rounded to power of two")
			require.Equal(t, uint64(0), rb.Tail(), "initial tail should be 0")
		})
	}
}

func TestRingBufferWrite(t *testing.T) {
	t.Run("write advances the tail by one", func(t *testing.T) {
		rb := ring.NewRingBuffer[int](8)
		msg := 42

		rb.Write(msg)
		require.Equal(t, uint64(1), rb.Tail(), "first write should be seq 1")

		rb.Write(msg)
		require.Equal(t, uint64(2), rb.Tail(), "second write should be seq 2")
	})

	t.Run("write stores message in correct slot", func(t *testing.T) {
		rb := ring.NewRingBuffer[int](4)
		msg1 := 10
		msg2 := 20

		rb.Write(msg1)
		rb.Write(msg2)

		data1, slotSeq1 := rb.Slot(1).Read()
		require.Equal(t, 10, data1)
		require.Equal(t, uint64(1), slotSeq1)

		data2, slotSeq2 := rb.Slot(2).Read()
		require.Equal(t, 20, data2)
		require.Equal(t, uint64(2), slotSeq2)
	})
}

func TestRingBufferSlot(t *testing.T) {
	t.Run("slot indexing wraps correctly", func(t *testing.T) {
		rb := ring.NewRingBuffer[int](4)

		// Sequences 1, 5 should map to same slot (index 1)
		slot1 := rb.Slot(1)
		slot5 := rb.Slot(5)
		require.Equal(t, slot1, slot5, "seq 1 and 5 should map to same slot")

		// Sequences 2, 6 should map to same slot (index 2)
		slot2 := rb.Slot(2)
		slot6 := rb.Slot(6)
		require.Equal(t, slot2, slot6, "seq 2 and 6 should map to same slot")
	})

	t.Run("slot returns correct slot for sequence", func(t *testing.T) {
		rb := ring.NewRingBuffer[int](8)
		msg := 42

		rb.Write(msg)
		slot := rb.Slot(rb.Tail())

		data, slotSeq := slot.Read()
		require.Equal(t, 42, data)
		require.Equal(t, rb.Tail(), slotSeq)
	})
}

func TestRingBufferTail(t *testing.T) {
	rb := ring.NewRingBuffer[int](8)
	require.Equal(t, uint64(0), rb.Tail(), "initial tail should be 0")

	msg := 42
	rb.Write(msg)
	require.Equal(t, uint64(1), rb.Tail(), "tail should be 1 after one write")

	rb.Write(msg)
	require.Equal(t, uint64(2), rb.Tail(), "tail should be 2 after two writes")
}

// TestBasicIteratorNoLag tests basic iteration without lag
func TestBasicIteratorNoLag(t *testing.T) {
	numEvents := 100
	rb := ring.NewRingBuffer[int](uint64(numEvents))

	// Pin the start sequence at 1 before the producer can move the tail.
	events := rb.Iterator(t.Context())

	// Produce messages
	go func() {
		for i := range numEvents {
			msg := i
			rb.Write(msg)
		}
	}()

	// Consume messages
	received := 0
	for ev := range events {
		val, ok := ev.AsEvent()
		require.True(t, ok)
		require.Equal(t, received, val, "should receive messages in order")
		received++
		if received >= numEvents {
			break
		}
	}
	require.Equal(t, numEvents, received, "should receive all messages")
}

// TestMultipleIteratorsReceiveSameData tests that multiple iterators receive the same data
func TestMultipleIteratorsReceiveSameData(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		numEvents := 1000
		numIterators := 100
		rb := ring.NewRingBuffer[int](uint64(numEvents))

		// Start iterators
		var wg sync.WaitGroup
		for range numIterators {
			wg.Go(func() {
				received := 0
				for ev := range rb.Iterator(t.Context()) {
					val, ok := ev.AsEvent()
					require.True(t, ok)
					require.Equal(t, received, val)
					received++
					if received >= numEvents {
						break
					}
				}
			})
		}

		synctest.Wait()
		// Publish messages
		for i := range numEvents {
			rb.Write(i)
		}

		// Wait for all iterators to complete. A stalled reader leaves every goroutine
		// in the bubble durably blocked, which synctest reports as a deadlock.
		wg.Wait()
	})
}

// TestConcurrentWrites_AllDelivered_NoLag tests concurrent writes with no lag
func TestConcurrentWrites_AllDelivered_NoLag(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		type payload struct {
			Prod int
			Seq  int
		}
		M := 8                    // producers
		K := 500                  // messages per producer
		capacity := uint64(M * K) // ensure no overwrite
		rb := ring.NewRingBuffer[payload](capacity)

		// Consume in a goroutine that starts before any write, so the iterator pins
		// its start sequence at 1 (Tail+1). The iterator only delivers sequences newer
		// than the tail at the moment iteration begins; draining after all writes would
		// start at Tail+1 and block forever.
		seen := make([]int, M)
		for i := range M {
			seen[i] = -1
		}
		total := M * K
		var consumer sync.WaitGroup
		consumer.Go(func() {
			received := 0
			for ev := range rb.Iterator(t.Context()) {
				p, ok := ev.AsEvent()
				require.True(t, ok, "received lag")
				require.Equal(t, seen[p.Prod]+1, p.Seq, "per-producer order must be preserved")
				seen[p.Prod] = p.Seq
				received++
				if received >= total {
					return
				}
			}
		})

		// Let the iterator pin its start sequence before producing.
		synctest.Wait()

		// Launch producers
		var wg sync.WaitGroup
		for pid := range M {
			wg.Go(func() {
				for i := range K {
					msg := payload{Prod: pid, Seq: i}
					rb.Write(msg)
				}
			})
		}
		wg.Wait()
		consumer.Wait()
	})
}

// TestOverwriteMarksOldestSequence checks the overwrite invariant directly on the slots:
// once the ring has lapped, Tail-Capacity+1 is the oldest sequence still readable. Lag
// delivery through an iterator is covered by TestIteratorLagResync.
func TestOverwriteMarksOldestSequence(t *testing.T) {
	rb := ring.NewRingBuffer[int](2) // capacity 2

	// Write 2 messages (seq 1, 2)
	for i := 1; i <= 2; i++ {
		msg := i
		rb.Write(msg)
	}

	// Write 3 more messages (seq 3, 4, 5) causing overwrite
	// With capacity 2, seq 1-2 are overwritten, oldest is now 4
	for i := 3; i <= 5; i++ {
		msg := i
		rb.Write(msg)
	}

	// Verify overwrite occurred
	slot1 := rb.Slot(1) // seq 1 slot should be overwritten
	_, seq1 := slot1.Read()
	require.GreaterOrEqual(t, seq1, uint64(3), "slot 1 should be overwritten")

	// The oldest still-valid sequence is tail-capacity+1 (computed from public state).
	// Continuation invariant: slot at `oldest` still holds `oldest`, while the slot
	// for `oldest-1` has been overwritten by a later seq.
	oldest := rb.Tail() - rb.Capacity() + 1
	require.Equal(t, uint64(4), oldest, "oldest should be tail-capacity+1 = 5-2+1 = 4")

	_, oldestSlotSeq := rb.Slot(oldest).Read()
	require.Equal(t, oldest, oldestSlotSeq, "slot at oldest must still hold that seq")

	_, preOldestSlotSeq := rb.Slot(oldest - 1).Read()
	require.Greater(t, preOldestSlotSeq, oldest-1, "slot before oldest must be overwritten")
}

// TestIteratorLagResync tests that iterator resyncs after lag
func TestIteratorLagResync(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rb := ring.NewRingBuffer[int](2) // capacity 2, mask 1

		// A freshly created iterator starts at Tail+1, so it can never lag on its
		// first read. To observe a lag we start consuming, deliver one event, pause
		// the consumer while the producer overwrites the ring, then resume: the next
		// read must surface a lag and the iterator must resync to the oldest sequence.
		proceed := make(chan struct{})
		received := make([]int, 0)
		var sawLag bool
		var consumer sync.WaitGroup
		consumer.Go(func() {
			first := true
			for ev := range rb.Iterator(t.Context()) {
				if p, ok := ev.AsEvent(); ok {
					received = append(received, p)
					if first {
						first = false
						<-proceed // block so the producer can overwrite the ring
					}
				} else if lag, ok := ev.AsLag(); ok {
					sawLag = true
					require.GreaterOrEqual(t, lag.NextSeq, uint64(4), "next seq should be at least 4")
				} else {
					t.Errorf("should have event or lag")
				}
				if len(received) >= 3 {
					return
				}
			}
		})

		// Let the iterator pin its start sequence at 1 before the first write.
		synctest.Wait()

		// Deliver seq 1; the consumer reads it and then blocks on proceed.
		one := 1
		rb.Write(one)
		synctest.Wait()

		// Overwrite the ring while the consumer is paused (seq 2..5, capacity 2).
		for i := 2; i <= 5; i++ {
			msg := i
			rb.Write(msg)
		}
		close(proceed) // resume: next read lags, then resyncs to the oldest sequence

		consumer.Wait()

		require.True(t, sawLag, "should have observed a lag notification")
		require.GreaterOrEqual(t, len(received), 3, "should receive at least 3 messages")
	})
}

// TestIteratorContextCancellation tests that iterator stops on context cancellation
func TestIteratorContextCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rb := ring.NewRingBuffer[int](8)
		ctx, cancel := context.WithCancel(t.Context())

		var reader sync.WaitGroup
		reader.Go(func() {
			for range rb.Iterator(ctx) {
			}
		})

		// Let the iterator reach its first wait.
		synctest.Wait()

		// Cancel context
		cancel()

		// The iterator must return; if it does not, the bubble deadlocks and fails.
		reader.Wait()
	})
}

// TestIteratorCancellationRacesWrites cancels many readers while a writer keeps
// publishing, so cancel lands at every point of the wait loop (see RingBuffer.wakeWaiters).
func TestIteratorCancellationRacesWrites(t *testing.T) {
	const readers = 64
	rb := ring.NewRingBuffer[int](8)

	writerCtx, stopWriter := context.WithCancel(t.Context())
	var writer sync.WaitGroup
	defer func() {
		stopWriter()
		writer.Wait()
	}()
	writer.Go(func() {
		for i := 0; writerCtx.Err() == nil; i++ {
			rb.Write(i)
		}
	})

	var wg sync.WaitGroup
	for reader := range readers {
		// Every deadline is short but nonzero, so each reader starts on a live context
		// and is cancelled mid-flight rather than before it begins. Spacing them spreads
		// the cancels across a few ms of writer activity, hitting varied wait-queue
		// states.
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(t.Context(), time.Duration(reader+1)*50*time.Microsecond)
			defer cancel()
			for range rb.Iterator(ctx) {
			}
		})
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("a cancelled iterator did not exit")
	}
}

// TestIteratorContextCancellation_AlreadyCancelled tests that an iterator returns
// immediately when the context is cancelled before iteration begins.
func TestIteratorContextCancellation_AlreadyCancelled(t *testing.T) {
	rb := ring.NewRingBuffer[int](8)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	done := make(chan struct{})
	go func() {
		for range rb.Iterator(ctx) {
			t.Error("iterator should not yield on cancelled context")
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("iterator should exit immediately on already-cancelled context")
	}
}

// TestIteratorContextCancellation_BacklogAvailable pins that a cancelled ctx stops the
// iterator even when its next sequence is already in the ring, so the reader never blocks.
func TestIteratorContextCancellation_BacklogAvailable(t *testing.T) {
	t.Run("cancelled before the first read", func(t *testing.T) {
		rb := ring.NewRingBuffer[int](8)
		ctx, cancel := context.WithCancel(t.Context())

		it := rb.Iterator(ctx)
		for i := range 5 {
			rb.Write(i)
		}
		cancel()

		for range it {
			t.Fatal("iterator should not yield a backlogged value on a cancelled context")
		}
	})

	t.Run("cancelled mid-backlog", func(t *testing.T) {
		rb := ring.NewRingBuffer[int](8)
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		it := rb.Iterator(ctx)
		for i := range 5 {
			rb.Write(i)
		}

		delivered := 0
		for range it {
			delivered++
			cancel()
		}

		require.Equal(t, 1, delivered, "iterator should stop on the iteration after cancel")
	})
}

// TestIteratorBreakStopsIteration tests that breaking from loop stops iterator
func TestIteratorBreakStopsIteration(t *testing.T) {
	rb := ring.NewRingBuffer[int](16)

	// Pin the subscription before writing, then consume: the iterator's start
	// position is fixed at call time, so these writes are all delivered.
	it := rb.Iterator(t.Context())
	for i := 1; i <= 10; i++ {
		msg := i
		rb.Write(msg)
	}

	count := 0
	for ev := range it {
		if _, ok := ev.AsEvent(); ok {
			count++
			if count >= 5 {
				break // Break out of loop
			}
		}
	}

	require.Equal(t, 5, count, "should have iterated 5 times before breaking")
}

// TestIteratorSlowConsumerDoesNotDeadlock tests that slow consumer doesn't deadlock
func TestIteratorSlowConsumerDoesNotDeadlock(t *testing.T) {
	const (
		bufferSize = uint64(64)
		bursts     = 30
		perBurst   = 20
	)
	lastMessage := (bursts-1)*1000 + perBurst - 1
	rb := ring.NewRingBuffer[int](bufferSize)

	// The consumer stops once it has caught up with the producer's final message, so the
	// context is only a backstop: were a lap ever missed, the consumer would park on an
	// overwritten sequence forever, and this bounds that failure instead of hanging.
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	// A slow consumer on a single subscription keeps receiving the backlog, as events or
	// as lag notifications once it falls behind, instead of dropping it by re-subscribing
	// at the tail.
	var recvCount uint64
	var sawLag bool
	var wg sync.WaitGroup
	wg.Go(func() {
		for ev := range rb.Iterator(ctx) {
			time.Sleep(2 * time.Millisecond) // simulate slow processing
			recvCount++
			if _, ok := ev.AsLag(); ok {
				sawLag = true
			}
			if v, ok := ev.AsEvent(); ok && v == lastMessage {
				return
			}
		}
	})

	// Producer sends bursts far faster than the consumer can drain them.
	for b := range bursts {
		for i := range perBurst {
			rb.Write(b*1000 + i)
		}
		time.Sleep(1 * time.Millisecond)
	}

	wg.Wait()
	require.True(t, sawLag, "a consumer this slow must be told it was lapped")
	require.GreaterOrEqual(t, recvCount, bufferSize,
		"consumer must make progress despite slow processing")
}

// TestIteratorConcurrentReadWrite tests multiple iterators reading concurrently
func TestIteratorConcurrentReadWrite(t *testing.T) {
	rb := ring.NewRingBuffer[int](8)
	numIterators := 5

	// Move the tail off zero. Readers pin at Tail+1, so none of these are visible to
	// them; they exist so the readers start mid-stream rather than at sequence 1.
	for i := 1; i <= 10; i++ {
		rb.Write(i)
	}

	// Pin every reader's start sequence before the batch they do read, so each one is
	// guaranteed the five events it waits for.
	iters := make([]iter.Seq[ring.EventOrLag[int]], numIterators)
	for i := range iters {
		iters[i] = rb.Iterator(t.Context())
	}

	var wg sync.WaitGroup

	for _, events := range iters {
		wg.Go(func() {
			count := 0
			for ev := range events {
				if _, ok := ev.AsEvent(); ok {
					count++
					if count >= 5 {
						break
					}
				}
			}
		})
	}

	// Write more messages while iterators are reading
	for i := 11; i <= 20; i++ {
		msg := i
		rb.Write(msg)
		time.Sleep(1 * time.Millisecond)
	}

	// Wait for iterators to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Good, all iterators completed
	case <-time.After(2 * time.Second):
		t.Fatal("iterators should complete")
	}
}

// TestSlotWriteKeepsNewerSequence pins the rule that lets Write run without a global lock:
// a writer that arrives after a later sequence has landed in its slot must not regress it.
func TestSlotWriteKeepsNewerSequence(t *testing.T) {
	rb := ring.NewRingBuffer[int](4)
	slot := rb.Slot(1)

	slot.Write(10, 5)
	slot.Write(20, 1) // lapped writer, same slot, older sequence

	data, seq := slot.Read()
	require.Equal(t, uint64(5), seq)
	require.Equal(t, 10, data)
}

// Sinks for TestZeroAllocations. Unlike testing.B.Loop, which keeps call results alive
// itself, testing.AllocsPerRun only diffs the allocation counter around f: storing to a
// package-level variable is what stops a pure call from being optimised out of the guard.
var (
	sinkUnion ring.EventOrLag[int]
	sinkLag   ring.LaggedError
	sinkErr   error
	sinkInt   int
	sinkSeq   uint64
	sinkOK    bool
)

// TestZeroAllocations guards the allocation-free claims in this package. EventOrLag is a
// struct with a discriminator rather than an interface, and the publish and read paths
// store and return T directly, so nothing on these paths boxes. Iterator is excluded: it
// builds a closure and registers a context hook, so it allocates by construction.
func TestZeroAllocations(t *testing.T) {
	rb := ring.NewRingBuffer[int](1024)
	slot := rb.Slot(1)
	event := ring.NewEvent(42)
	lag := ring.NewLag[int](1, 2)
	var seq uint64

	// The two writers mutate shared state, so they survive on their own. The rest are
	// pure and would be eliminated if their results went to _, leaving the guard to
	// measure nothing.
	paths := map[string]func(){
		"RingBuffer.Write": func() { rb.Write(42) },
		"Slot.Write":       func() { seq++; slot.Write(42, seq) },
		"Slot.Read":        func() { sinkInt, sinkSeq = slot.Read() },
		"NewEvent":         func() { sinkUnion = ring.NewEvent(42) },
		"NewLag":           func() { sinkUnion = ring.NewLag[int](1, 2) },
		"AsEvent":          func() { sinkInt, sinkOK = event.AsEvent() },
		"AsLag":            func() { sinkLag, sinkOK = lag.AsLag() },
		"Err":              func() { sinkErr = lag.Err() },
	}
	for name, path := range paths {
		t.Run(name, func(t *testing.T) {
			require.Zero(t, testing.AllocsPerRun(100, path))
		})
	}
}

// TestParallelWritersNeverRegressSlots runs many writers with no coordination and checks
// that every slot ends up holding the highest sequence assigned to it.
func TestParallelWritersNeverRegressSlots(t *testing.T) {
	const (
		capacity        = uint64(8)
		writers         = 8
		writesPerWriter = 20000
		total           = uint64(writers * writesPerWriter)
	)
	rb := ring.NewRingBuffer[int](capacity)

	var wg sync.WaitGroup
	for w := range writers {
		wg.Go(func() {
			for range writesPerWriter {
				rb.Write(w)
			}
		})
	}
	wg.Wait()

	require.Equal(t, total, rb.Tail())
	for i := range capacity {
		_, seq := rb.Slot(i).Read()
		// highest k <= total with k % capacity == i
		want := total - (total-i)%capacity
		require.Equal(t, want, seq, "slot %d", i)
	}
}
