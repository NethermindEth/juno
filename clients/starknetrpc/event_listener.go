package starknetrpc

import "time"

type EventListener interface {
	OnResponse(method, outcome string, took time.Duration)
}

type SelectiveListener struct {
	OnResponseCb func(method, outcome string, took time.Duration)
}

func (l *SelectiveListener) OnResponse(method, outcome string, took time.Duration) {
	if l.OnResponseCb != nil {
		l.OnResponseCb(method, outcome, took)
	}
}
