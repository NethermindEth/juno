package db

import "time"

type EventListener interface {
	OnIO(write bool, duration time.Duration)
	OnCommit(duration time.Duration)
}

type SelectiveListener struct {
	OnIOCb     func(write bool, duration time.Duration)
	OnCommitCb func(duration time.Duration)
}

func (l *SelectiveListener) OnIO(write bool, duration time.Duration) {
	if l.OnIOCb != nil {
		l.OnIOCb(write, duration)
	}
}

func (l *SelectiveListener) OnCommit(duration time.Duration) {
	if l.OnCommitCb != nil {
		l.OnCommitCb(duration)
	}
}
