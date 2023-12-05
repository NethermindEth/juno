package jsonrpc

import (
	"net"
	"time"
)

type NewRequestListener interface {
	OnNewRequest(method string)
	OnNewConnection(conn net.Conn)
	OnDisconnect(conn net.Conn)
}

type EventListener interface {
	NewRequestListener
	OnRequestHandled(method string, took time.Duration)
	OnRequestFailed(method string, data any)
}

type SelectiveListener struct {
	OnNewRequestCb     func(method string)
	OnNewConnectionCb  func(conn net.Conn)
	OnDisconnectCb     func(conn net.Conn)
	OnRequestHandledCb func(method string, took time.Duration)
	OnRequestFailedCb  func(method string, data any)
}

func (l *SelectiveListener) OnNewRequest(method string) {
	if l.OnNewRequestCb != nil {
		l.OnNewRequestCb(method)
	}
}

func (l *SelectiveListener) OnNewConnection(conn net.Conn) {
	if l.OnNewConnectionCb != nil {
		l.OnNewConnectionCb(conn)
	}
}

func (l *SelectiveListener) OnDisconnect(conn net.Conn) {
	if l.OnDisconnectCb != nil {
		l.OnDisconnectCb(conn)
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
