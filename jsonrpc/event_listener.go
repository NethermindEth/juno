package jsonrpc

import "time"

type NewRequestListener interface {
	OnNewRequest(method string)
}

type EventListener interface {
	NewRequestListener
	OnRequestHandled(method string, took time.Duration)
	OnRequestFailed(method string, data any)
}

type SelectiveListener struct {
	OnNewRequestCb     func(method string)
	OnRequestHandledCb func(method string, took time.Duration)
	OnRequestFailedCb  func(method string, data any)
}

func (l *SelectiveListener) OnNewRequest(method string) {
	if l.OnNewRequestCb != nil {
		l.OnNewRequestCb(method)
	}
}

func (l *SelectiveListener) OnRequestHandled(method string, took time.Duration) {
	if l.OnRequestHandledCb != nil {
		l.OnRequestHandledCb(method, took)
	}
}

func (l *SelectiveListener) OnRequestFailed(method string, data any) {
	if l.OnRequestFailedCb != nil {
		l.OnRequestFailedCb(method, data)
	}
}
