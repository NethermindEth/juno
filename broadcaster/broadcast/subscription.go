package broadcast

import (
	"context"
	"iter"

	"github.com/NethermindEth/juno/broadcaster/broadcast/ring"
)

// Subscribable is a read-only handle over a Broadcast's ring. It is a value type and safe to copy.
type Subscribable[T any] struct {
	ring *ring.RingBuffer[T]
}

// Subscribe starts a subscription delivering raw ring.EventOrLag[T] values.
func (s Subscribable[T]) Subscribe() *Subscription[ring.EventOrLag[T]] {
	//nolint:gosec // G118: cancel is stored and called by Unsubscribe
	ctx, cancel := context.WithCancel(context.Background())
	return &Subscription[ring.EventOrLag[T]]{
		out:    IterToChan(ctx, s.ring.Iterator(ctx), 1),
		cancel: cancel,
	}
}

// SubscribeWith starts a subscription delivering O values: transform maps the ring's
// EventOrLag[T] iterator to any output iterator, for instance a lag policy yielding T
// or a mapper yielding a derived type.
func (s Subscribable[T]) SubscribeWith[O any](
	transform func(iter.Seq[ring.EventOrLag[T]]) iter.Seq[O],
) *Subscription[O] {
	//nolint:gosec // G118: cancel is stored and called by Unsubscribe
	ctx, cancel := context.WithCancel(context.Background())
	return &Subscription[O]{
		out:    IterToChan(ctx, transform(s.ring.Iterator(ctx)), 1),
		cancel: cancel,
	}
}

// Subscription is a single consumer delivering O values on Recv until Unsubscribe.
type Subscription[O any] struct {
	out    <-chan O
	cancel context.CancelFunc
}

// Recv returns the channel delivering this subscription's values; it is closed after Unsubscribe.
func (sub *Subscription[O]) Recv() <-chan O {
	return sub.out
}

// Unsubscribe stops delivery. Cancelling the context fires the iterator's AfterFunc, which
// wakes a blocked reader so the goroutine exits (see ring.RingBuffer.wakeWaiters). Idempotent.
func (sub *Subscription[O]) Unsubscribe() {
	sub.cancel()
}

// IterToChan pumps seq into a channel of the given buffer size from a goroutine; the channel
// is closed when seq ends or ctx is done, and a blocked send gives up when ctx is done.
func IterToChan[T any](ctx context.Context, seq iter.Seq[T], bufferSize int) <-chan T {
	out := make(chan T, bufferSize)
	go func() {
		defer close(out)
		for value := range seq {
			select {
			case out <- value:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}
