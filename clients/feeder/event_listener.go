package feeder

import "time"

type EventListener interface {
	OnResponse(urlPath string, status int, took time.Duration)
}

type SelectiveListener struct {
	OnResponseCb func(urlPath string, status int, took time.Duration)
}

func (l *SelectiveListener) OnResponse(urlPath string, status int, took time.Duration) {
	if l.OnResponseCb != nil {
		l.OnResponseCb(urlPath, status, took)
	}
}
