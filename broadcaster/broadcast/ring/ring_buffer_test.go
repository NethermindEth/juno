package ring_test

import (
	"context"
	"runtime"
	"sync"
	"testing"
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
	t.Run("write returns sequence numbers", func(t *testing.T) {
		rb := ring.NewRingBuffer[int](8)
		msg := 42

		seq1 := rb.Write(msg)
		require.Equal(t, uint64(1), seq1, "first write should return seq 1")

		seq2 := rb.Write(msg)
		require.Equal(t, uint64(2), seq2, "second write should return seq 2")

		require.Equal(t, uint64(2), rb.Tail(), "tail should be 2")
	})

	t.Run("write stores message in correct slot", func(t *testing.T) {
		rb := ring.NewRingBuffer[int](4)
		msg1 := 10
		msg2 := 20

		seq1 := rb.Write(msg1)
		seq2 := rb.Write(msg2)

		data1, slotSeq1 := rb.Slot(seq1).Read()
		require.Equal(t, 10, data1)
		require.Equal(t, seq1, slotSeq1)

		data2, slotSeq2 := rb.Slot(seq2).Read()
		require.Equal(t, 20, data2)
		require.Equal(t, seq2, slotSeq2)
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

		seq := rb.Write(msg)
		slot := rb.Slot(seq)

		data, slotSeq := slot.Read()
		require.Equal(t, 42, data)
		require.Equal(t, seq, slotSeq)
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

	// Produce messages
	go func() {
		for i := range numEvents {
			msg := i
			rb.Write(msg)
		}
	}()

	// Consume messages
	received := 0
	for ev := range rb.Iterator(t.Context()) {
		require.True(t, ev.IsEvent())
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

	time.Sleep(20 * time.Millisecond)
	// Publish messages
	for i := range numEvents {
		msg := i
		rb.Write(msg)
	}

	// Wait for all iterators to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All iterators completed
	case <-time.After(5 * time.Second):
		t.Fatal("iterators should complete")
	}
}

// TestConcurrentWrites_AllDelivered_NoLag tests concurrent writes with no lag
func TestConcurrentWrites_AllDelivered_NoLag(t *testing.T) {
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
	done := make(chan struct{})
	go func() {
		defer close(done)
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
	}()

	// Give the iterator time to pin its start sequence before producing.
	time.Sleep(20 * time.Millisecond)

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

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("consumer should receive all messages")
	}
}

// TestIteratorLagDetection tests that iterator detects lag when overwrite occurs
func TestIteratorLagDetection(t *testing.T) {
	rb := ring.NewRingBuffer[int](2) // capacity 2

	// Write 2 messages (seq 1, 2)
	for i := 1; i <= 2; i++ {
		msg := i
		rb.Write(msg)
	}

	// Wait a bit for iterator to potentially start
	time.Sleep(5 * time.Millisecond)

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
	rb := ring.NewRingBuffer[int](2) // capacity 2, mask 1

	// A freshly created iterator starts at Tail+1, so it can never lag on its
	// first read. To observe a lag we start consuming, deliver one event, pause
	// the consumer while the producer overwrites the ring, then resume: the next
	// read must surface a lag and the iterator must resync to the oldest sequence.
	proceed := make(chan struct{})
	received := make([]int, 0)
	var sawLag bool
	done := make(chan struct{})
	go func() {
		defer close(done)
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
	}()

	// Let the iterator pin its start sequence at 1 before the first write.
	time.Sleep(20 * time.Millisecond)

	// Deliver seq 1; the consumer reads it and then blocks on proceed.
	one := 1
	rb.Write(one)
	time.Sleep(10 * time.Millisecond)

	// Overwrite the ring while the consumer is paused (seq 2..5, capacity 2).
	for i := 2; i <= 5; i++ {
		msg := i
		rb.Write(msg)
	}
	close(proceed) // resume: next read lags, then resyncs to the oldest sequence

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("consumer should resync and continue reading after lag")
	}

	require.True(t, sawLag, "should have observed a lag notification")
	require.GreaterOrEqual(t, len(received), 3, "should receive at least 3 messages")
}

// TestIteratorContextCancellation tests that iterator stops on context cancellation
func TestIteratorContextCancellation(t *testing.T) {
	rb := ring.NewRingBuffer[int](8)
	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan bool, 1)
	go func() {
		count := 0
		for ev := range rb.Iterator(ctx) {
			if ev.IsEvent() {
				count++
			}
		}
		done <- true
	}()

	// Give iterator time to start waiting
	time.Sleep(10 * time.Millisecond)

	// Cancel context
	cancel()

	select {
	case <-done:
		// Good, iterator stopped
	case <-time.After(1 * time.Second):
		t.Fatal("iterator should stop on context cancellation")
	}
}

// TestIteratorCancellationRacesWrites cancels many readers while a writer keeps
// publishing, so cancel lands at every point of the wait loop (see RingBuffer.wakeWaiters).
func TestIteratorCancellationRacesWrites(t *testing.T) {
	const readers = 64
	rb := ring.NewRingBuffer[int](8)

	writerCtx, stopWriter := context.WithCancel(t.Context())
	defer stopWriter()
	go func() {
		for i := 0; writerCtx.Err() == nil; i++ {
			rb.Write(i)
			if i%16 == 0 {
				runtime.Gosched()
			}
		}
	}()

	var wg sync.WaitGroup
	for reader := range readers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithCancel(t.Context())
			go func() {
				time.Sleep(time.Duration(reader) * 50 * time.Microsecond)
				cancel()
			}()
			for range rb.Iterator(ctx) {
			}
		}()
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
		if ev.IsEvent() {
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
	bufferSize := uint64(64)
	rb := ring.NewRingBuffer[int](bufferSize)

	// The iterator blocks in waitForSequence, so a bounded context is the consumer's
	// escape hatch (the equivalent of the channel select's timeout branch): it unblocks
	// the range loop once the producer stops instead of parking forever.
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	// Slow consumer draining a single subscription, so it keeps receiving the backlog
	// (as events, or lag notifications when it falls behind) instead of dropping it by
	// re-subscribing at the tail.
	var recvCount uint64
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range rb.Iterator(ctx) {
			// Simulate slow processing
			time.Sleep(2 * time.Millisecond)
			recvCount++
			if recvCount >= 300 {
				return
			}
		}
	}()

	// Producer sends bursts
	sent := 0
	for b := range 30 {
		for i := range 20 {
			msg := b*1000 + i
			rb.Write(msg)
			sent++
		}
		time.Sleep(1 * time.Millisecond)
	}

	wg.Wait()
	require.GreaterOrEqual(t, recvCount, bufferSize,
		"consumer must make progress despite slow processing")
}

// TestIteratorConcurrentReadWrite tests multiple iterators reading concurrently
func TestIteratorConcurrentReadWrite(t *testing.T) {
	rb := ring.NewRingBuffer[int](8)
	numIterators := 5

	// Write initial messages
	for i := 1; i <= 10; i++ {
		msg := i
		rb.Write(msg)
	}

	var wg sync.WaitGroup
	wg.Add(numIterators)

	for range numIterators {
		go func() {
			defer wg.Done()
			count := 0
			for ev := range rb.Iterator(t.Context()) {
				if ev.IsEvent() {
					count++
					if count >= 5 {
						break
					}
				}
			}
		}()
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
