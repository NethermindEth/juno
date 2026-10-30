package ring

import "fmt"

// EventOrLag is a zero-allocation tagged union that represents either a regular event
// or a lag notification. It uses a struct with a discriminator instead of an interface
// to avoid interface allocations in the hot path.
type EventOrLag[T any] struct {
	// event holds the event value when isLag is false
	event T
	// lag holds the lag notification when isLag is true
	lag LaggedError
	// isLag discriminator: false = event, true = lag
	isLag bool
}

// NewEvent creates a new [EventOrLag] containing an event.
// Zero allocation - returns struct value.
func NewEvent[T any](event T) EventOrLag[T] {
	return EventOrLag[T]{
		event: event,
		isLag: false,
	}
}

// NewLag creates a new [EventOrLag] containing a lag notification.
// Zero allocation - returns struct value.
func NewLag[T any](missedSeq, nextSeq uint64) EventOrLag[T] {
	return EventOrLag[T]{
		lag: LaggedError{
			MissedSeq: missedSeq,
			NextSeq:   nextSeq,
		},
		isLag: true,
	}
}

// AsEvent returns the event and true, or the zero value and false if this holds a lag
// notification instead.
func (e *EventOrLag[T]) AsEvent() (T, bool) {
	if !e.isLag {
		return e.event, true
	}
	var zero T
	return zero, false
}

// AsLag returns the lag notification and true, or the zero [LaggedError] and false if this
// holds an event instead.
func (e *EventOrLag[T]) AsLag() (LaggedError, bool) {
	if e.isLag {
		return e.lag, true
	}
	return LaggedError{}, false
}

// Err returns the lag error if this is a lag notification, nil otherwise.
// This method allocates a *[LaggedError] when returning an error; for zero allocation use
// [EventOrLag.AsLag], which copies the embedded value instead.
func (e *EventOrLag[T]) Err() error {
	if !e.isLag {
		return nil
	}
	return &e.lag
}

// LaggedError indicates the requested sequence was overwritten by newer writes.
// - MissedSeq: the requested sequence that was lost.
// - NextSeq: the oldest sequence still available (resume point).
type LaggedError struct {
	MissedSeq uint64 // The sequence the subscriber attempted to read
	NextSeq   uint64 // The oldest available sequence in the buffer (where subscriber resumes)
}

func (e *LaggedError) Error() string {
	return fmt.Sprintf(
		"subscriber lagged: missed seq=%d, next available seq=%d",
		e.MissedSeq,
		e.NextSeq,
	)
}

func (e *LaggedError) Unwrap() error {
	return nil
}
