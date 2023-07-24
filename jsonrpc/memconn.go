package jsonrpc

import (
	"context"
	"errors"
	"io"
)

// MemConn is a simple in-memory JSON RPC connection.
type MemConn struct {
	ctx  context.Context
	send chan<- []byte
	recv <-chan []byte
}

func NewMemConn(ctx context.Context, send chan<- []byte, recv <-chan []byte) *MemConn {
	return &MemConn{
		ctx:  ctx,
		send: send,
		recv: recv,
	}
}

func (m *MemConn) Read(p []byte) (int, error) {
	select {
	case <-m.ctx.Done():
		return 0, ErrRead{
			Inner: m.ctx.Err(),
		}
	case msg, ok := <-m.recv:
		if !ok {
			return 0, io.EOF
		}
		return copy(p, msg), nil
	}
}

func (m *MemConn) Write(p []byte) (l int, err error) {
	defer func() {
		if panicked := recover(); panicked != nil {
			l = 0
			err = errors.New("memconn: write on closed connection")
		}
	}()
	m.send <- p
	l = len(p)
	err = nil
	return
}
