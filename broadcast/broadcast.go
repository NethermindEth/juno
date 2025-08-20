// Package broadcast implements fan-out event streaming and notification systems
// where multiple consumers need to receive all messages or be informed about lost messages,
// while the producers manages a bounded buffer with overwrites.
// Safe for multi-producer multi-consumer setting.
package broadcast

import (
	"errors"
	"sync"
	"sync/atomic"
)

// EventOrLag is a tagged union:
//   - If isLag is false, event holds a regular message.
//   - If isLag is true, lag holds the lag notification.
type EventOrLag[T any] struct {
	event T           // For a regular message
	lag   LaggedError // For a lag notification
	isLag bool
}

// IsEvent returns true if the EventOrLag contains a regular Event.
func (e *EventOrLag[T]) IsEvent() bool {
	return !e.isLag
}

// IsLag returns true if the EventOrLag contains a lag notification.
func (e *EventOrLag[T]) IsLag() bool {
	return e.isLag
}

// Event returns the event value or an error if none.
func (e *EventOrLag[T]) Event() (T, error) {
	var zero T
	if e.isLag {
		return zero, errors.New("no event present")
	}
	return e.event, nil
}

// Lag returns the lag info or an error if none.
func (e *EventOrLag[T]) Lag() (LaggedError, error) {
	if !e.isLag {
		return LaggedError{}, errors.New("no lag info present")
	}
	return e.lag, nil
}

// Subscription represents one consumer.
//   - bcast: back-reference to the Broadcast.
//   - nextSeq: next sequence to read.
//   - notifyC: 1-buffered wakeup channel (coalesced notifications).
//   - isClosed: ensures Unsubscribe runs only once.
//   - id: subscription id
//   - out: user-facing channel for events or lag notifications.
//   - done: closed by Unsubscribe; run observes it to abort blocking sends
type Subscription[T any] struct {
	bcast    *Broadcast[T]
	nextSeq  uint64
	notifyC  chan struct{}
	isClosed atomic.Bool
	id       uint64

	out  chan EventOrLag[T] // user-facing channel for messages
	done chan struct{}      // to shutdown goroutine
}

// Unsubscribe removes this subscriber from the broadcast (if still present) and
// closes its control channels.
//   - single-run via CAS.
//   - Deletes from subs under subMu.Lock, then closes done (to stop run).
func (sub *Subscription[T]) Unsubscribe() {
	if !sub.isClosed.CompareAndSwap(false, true) {
		return
	}
	sub.bcast.unsubscribe(sub.id)
	close(sub.done) // signal goroutine to exit
}

// run is the delivery goroutine for a subscription.
//   - Repeatedly tries to receive from ring at sub.nextSeq.
//   - On event: sends to out (blocks unless done is closed).
//   - On lag: updates sub.nextSeq to NextSeq and emits a lag notification.
//   - On not-yet-available (ErrFutureSeq): waits on notifyC or done.
//   - On close:
//     If broadcast is closed drain-on-close semantics on. Drains the buffer then exits and closes out.
//     If subscription is closed via Unsubscribe, returns immediately upon receiving done signal.
func (sub *Subscription[T]) run() {
	defer close(sub.out)
	defer sub.Unsubscribe()

	for {
		msg, err := sub.tryRecv()
		if err == nil {
			// Got msg! Deliver to user-facing channel (may block if user's channel is full)
			res := EventOrLag[T]{
				event: msg,
				isLag: false,
			}

			select {
			case sub.out <- res:
				continue
			case <-sub.done:
				return
			}
		}

		if errors.Is(err, ErrFutureSeq) {
			// Wait for new message or closure
			select {
			case <-sub.notifyC:
				// when channel is closed drain mode.
				// exit when drained the buffer or upon Unsubscribe.
				continue
			case <-sub.done:
				return
			}
		}

		var e *LaggedError
		if errors.As(err, &e) {
			sub.nextSeq = e.NextSeq
			// Lagged, notify consumer about lag
			res := EventOrLag[T]{
				lag:   *e,
				isLag: true,
			}
			select {
			case sub.out <- res:
				continue
			case <-sub.done:
				return
			}
		}
		// ErrClosed or unexpected error
		return
	}
}

// tryRecv fetches the next message if available:
//   - On success, returns (msg, nil) and increments nextSeq.
//   - If not yet available, returns ErrFutureSeq.
//   - If overwritten, returns LaggedError with the resume sequence.
//   - If broadcast is closed and nextSeq is not available (i.e., draining has reached newest),
//     returns ErrClosed to end the run loop.
func (sub *Subscription[T]) tryRecv() (T, error) {
	var zero T

	b := sub.bcast

	seq := sub.nextSeq

	msg, err := b.ring.Get(seq)

	if err == nil {
		sub.nextSeq++
		return msg, nil
	}

	// Drain-on-close semantics. Drains the buffer even if closed.
	if errors.Is(err, ErrFutureSeq) && b.closed.Load() {
		return zero, ErrClosed
	}

	return zero, err
}

// Recv returns the user-facing channel for this subscription.
func (sub *Subscription[T]) Recv() <-chan EventOrLag[T] {
	return sub.out
}

// Broadcast is the main fan-out structure.
// - ring: the shared ring buffer used by all senders/readers.
// - closed: atomic flag indicating broadcast has been closed.
// - subMu: protects the subs map and nextSubID.
// - subs: set of current subscriptions.
// - nextSubID: monotonically increasing subsctiption ID to assign to next subscriber.
type Broadcast[T any] struct {
	ring *ringBuffer[T]

	closed atomic.Bool

	subMu     sync.RWMutex
	subs      map[uint64]*Subscription[T]
	nextSubID uint64
}

// New constructs a Broadcast with a ring buffer of at least the given capacity
// (capacity is rounded up to the next power of two).
func New[T any](capacity uint64) *Broadcast[T] {
	return &Broadcast[T]{
		ring: newRingBuffer[T](capacity),
		subs: make(map[uint64]*Subscription[T]),
	}
}

// Send publishes msg to the ring and wakes subscribers.
//   - Returns ErrClosed if closed is already set (a concurrent Close after this check may still allow this Push).
//   - Push is serialised via ring’s tailMu.
//   - Wakes subscribers by iterating subs under subMu.RLock and attempting non-blocking sends on their notifyC.
func (b *Broadcast[T]) Send(msg T) error {
	if b.closed.Load() {
		return ErrClosed
	}
	// Push writes the data and advances tail under ring locks.
	b.ring.Push(msg)

	b.subMu.RLock()
	defer b.subMu.RUnlock()
	// Wake subscribers (only if someone is blocked)
	for _, sub := range b.subs {
		select {
		case sub.notifyC <- struct{}{}:
		default:
		}
	}
	return nil
}

// Subscribe adds a new subscriber starting at the current tail (next message).
// Subscription must be ended by calling Unsubscribe to terminate forwarder goroutine and cleanup resources,
// or after user facing channel is closed after draining the buffer when broadcast is closed.
//
// - Acquires subMu to add to the subs map.
// - Reads ring.tail under ring's tail lock to set starting sequence.
// - Spawns a goroutine to deliver messages to sub.out.
func (b *Broadcast[T]) Subscribe() *Subscription[T] {
	b.ring.tailMu.RLock()
	next := b.ring.tail
	b.ring.tailMu.RUnlock()

	b.subMu.Lock()
	subID := b.nextSubID
	b.nextSubID++
	sub := &Subscription[T]{
		bcast:   b,
		nextSeq: next, // next msg to read
		notifyC: make(chan struct{}, 1),
		out:     make(chan EventOrLag[T], 1),
		done:    make(chan struct{}),
		id:      subID,
	}

	b.subs[sub.id] = sub
	b.subMu.Unlock()
	go sub.run()

	return sub
}

// unsubscribe removes subscriber from subsribers map.
// Subsequent calls are no-op.
func (b *Broadcast[T]) unsubscribe(subId uint64) {
	b.subMu.Lock()
	delete(b.subs, subId)
	b.subMu.Unlock()
}

// Close marks the broadcast as closed (single-run via CAS),
// closes all subscribers notifyC channel(drain-mode), and clears the subs map.
// It does not close per-sub user facing channels here;
// each subscription’s run goroutine is responsible for exiting (draining up to the latest) and
// will close its out channel on return.
func (b *Broadcast[T]) Close() {
	if !b.closed.CompareAndSwap(false, true) {
		return
	}

	b.subMu.Lock()
	defer b.subMu.Unlock()

	for _, sub := range b.subs {
		close(sub.notifyC) // Drain mode
	}
	b.subs = make(map[uint64]*Subscription[T])
}
