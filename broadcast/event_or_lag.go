package broadcast

import "errors"

type Broadcast[T any] interface {
	Send(msg T) error
	Subscribe() Subscription[T]
	Close()
}

type Subscription[T any] interface {
	Recv() <-chan EventOrLag[T]
	Unsubscribe()
}

// Returned when the broadcast is closed and no more sends are possible.
var ErrClosed = errors.New("broadcast channel closed")

// EventOrLag is a tagged union:
//   - If isLag is false, event holds a regular message.
//   - If isLag is true, lag holds the lag notification.
type EventOrLag[T any] struct {
	event T          // For a regular message
	lag   LaggedInfo // For a lag notification
	isLag bool
}

// IsEvent returns true if the EventOrLag contains a regular Event.
func (e EventOrLag[T]) IsEvent() bool {
	return !e.isLag
}

// IsLag returns true if the EventOrLag contains a lag notification.
func (e EventOrLag[T]) IsLag() bool {
	return e.isLag
}

// Event returns the event value or an error if none.
func (e EventOrLag[T]) Event() (T, error) {
	var zero T
	if e.isLag {
		return zero, errors.New("no event present")
	}
	return e.event, nil
}

// Lag returns the lag info or an error if none.
func (e EventOrLag[T]) Lag() (LaggedInfo, error) {
	if !e.isLag {
		return LaggedInfo{}, errors.New("no lag info present")
	}
	return e.lag, nil
}

// LaggedInfo is a transport-friendly struct to deliver lag information to subscribers.
type LaggedInfo struct {
	MissedSeq uint64
	NextSeq   uint64
}
