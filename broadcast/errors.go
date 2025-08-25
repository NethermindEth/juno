package broadcast

import (
	"errors"
	"fmt"
)

var (
	// Returned when the broadcast is closed and no more sends are possible.
	ErrClosed = errors.New("broadcast channel closed")
	// Returned when the requested sequence is ahead of the latest published (not yet available).
	ErrFutureSeq = errors.New("requested sequence is not published yet")
	// Returned when the requested sequence is 0. Which is indicates unitialized sequence in this system.
	ErrInvalidSequence = errors.New("invalid sequence: 0")
	ErrNoEvent         = errors.New("no event present")
	ErrNoLag           = errors.New("no lag info present")
)

// LaggedError indicates the requested sequence was overwritten by newer writes.
// - MissedSeq: the requested sequence that was lost.
// - NextSeq: the oldest sequence still available (resume point).
type LaggedError struct {
	MissedSeq uint64 // The sequence the subscriber attempted to read
	NextSeq   uint64 // The oldest available sequence in the buffer (where subscriber resumes)
}

func (e *LaggedError) Error() string {
	return fmt.Sprintf("subscriber lagged: missed seq=%d, next available seq=%d", e.MissedSeq, e.NextSeq)
}
