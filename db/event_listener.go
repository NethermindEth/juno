package db

import "time"

type Listener interface {
	WithListener(listener EventListener) KeyValueStore
}

type EventListener interface {
	OnIO(write bool, start time.Time)
	OnCommit(start time.Time)
	OnWriteStall(isL0 bool, duration time.Duration)
}

type SelectiveListener struct {
	OnIOCb         func(write bool, duration time.Duration)
	OnCommitCb     func(duration time.Duration)
	OnWriteStallCb func(isL0 bool, duration time.Duration)
}

func (l *SelectiveListener) OnIO(write bool, start time.Time) {
	if l.OnIOCb != nil {
		l.OnIOCb(write, time.Since(start))
	}
}

func (l *SelectiveListener) OnCommit(start time.Time) {
	if l.OnCommitCb != nil {
		l.OnCommitCb(time.Since(start))
	}
}

func (l *SelectiveListener) OnWriteStall(isL0 bool, duration time.Duration) {
	if l.OnWriteStallCb != nil {
		l.OnWriteStallCb(isL0, duration)
	}
}
