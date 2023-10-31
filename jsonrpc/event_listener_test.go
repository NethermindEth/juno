package jsonrpc_test

import "time"

type CountingEventListener struct {
	OnNewRequestLogs      []string
	OnRequestHandledCalls []struct {
		method string
		took   time.Duration
	}
	OnRequestFailedCalls []struct {
		method string
		data   any
	}
}

func (l *CountingEventListener) OnNewRequest(method string) {
	l.OnNewRequestLogs = append(l.OnNewRequestLogs, method)
}

func (l *CountingEventListener) OnRequestHandled(method string, took time.Duration) {
	l.OnRequestHandledCalls = append(l.OnRequestHandledCalls, struct {
		method string
		took   time.Duration
	}{
		method: method,
		took:   took,
	})
}

func (l *CountingEventListener) OnRequestFailed(method string, data any) {
	l.OnRequestFailedCalls = append(l.OnRequestFailedCalls, struct {
		method string
		data   any
	}{
		method: method,
		data:   data,
	})
}
