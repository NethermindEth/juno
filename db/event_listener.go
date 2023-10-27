package db

import "time"

type EventListener interface {
	OnIO(write bool, duration time.Duration)
}

type SelectiveListener struct {
	OnIOCb func(write bool, duration time.Duration)
}

func (l *SelectiveListener) OnIO(write bool, duration time.Duration) {
	if l.OnIOCb != nil {
		l.OnIOCb(write, duration)
	}
}
