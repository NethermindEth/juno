package preconfirmed

import "time"

// Update kinds [EventListener.OnPollSucceeded] reports.
const (
	UpdateNoChange = "no_change"
	UpdateDelta    = "delta"
	UpdateFull     = "full"
)

// Reasons [EventListener.OnPollFailed] reports.
const (
	FailureNotFound    = "not_found"
	FailureRateLimited = "rate_limited"
	FailureError       = "error"
)

// EventListener is told how each pre-confirmed poll went. A tick skipped while the node is not
// at the tip reports nothing.
type EventListener interface {
	// OnPollSucceeded reports a poll that applied the latest update, its kind and how long the
	// poll took.
	OnPollSucceeded(update string, took time.Duration)
	// OnPollFailed reports a poll that did not apply the latest update.
	OnPollFailed(reason string)
	// OnBackfill reports a poll starting to backfill the blocks below the latest.
	OnBackfill()
}

type SelectiveListener struct {
	OnPollSucceededCb func(update string, took time.Duration)
	OnPollFailedCb    func(reason string)
	OnBackfillCb      func()
}

func (l *SelectiveListener) OnPollSucceeded(update string, took time.Duration) {
	if l.OnPollSucceededCb != nil {
		l.OnPollSucceededCb(update, took)
	}
}

func (l *SelectiveListener) OnPollFailed(reason string) {
	if l.OnPollFailedCb != nil {
		l.OnPollFailedCb(reason)
	}
}

func (l *SelectiveListener) OnBackfill() {
	if l.OnBackfillCb != nil {
		l.OnBackfillCb()
	}
}
