package jsonrpc

import (
	"time"
)

type NewConnectionListener interface {
	OnNewConnection()
	OnDisconnect()
}

type NewRequestListener interface {
	NewConnectionListener
	OnNewRequest(method string)
}

type EventListener interface {
	NewRequestListener
	OnRequestHandled(method string, took time.Duration)
	OnRequestFailed(method string, data any)
}

type SelectiveListener struct {
	OnNewRequestCb     func(method string)
	OnNewConnectionCb  func()
	OnDisconnectCb     func()
	OnRequestHandledCb func(method string, took time.Duration)
	OnRequestFailedCb  func(method string, data any)
}

func (l *SelectiveListener) OnNewRequest(method string) {
	if l.OnNewRequestCb != nil {
		l.OnNewRequestCb(method)
	}
}

func (l *SelectiveListener) OnNewConnection() {
	if l.OnNewConnectionCb != nil {
		l.OnNewConnectionCb()
	}
}

func (l *SelectiveListener) OnDisconnect() {
	if l.OnDisconnectCb != nil {
		l.OnDisconnectCb()
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
