package jsonrpc

import "io"

// MemConn is a simple in-memory JSON RPC connection.
type MemConn struct {
	send chan<- []byte
	recv <-chan []byte
}

func NewMemConn(send chan<- []byte, recv <-chan []byte) *MemConn {
	return &MemConn{
		send: send,
		recv: recv,
	}
}

func (m *MemConn) Read(p []byte) (int, error) {
	msg, ok := <-m.recv
	if !ok {
		return 0, io.EOF
	}
	return copy(p, msg), nil
}

func (m *MemConn) Write(p []byte) (l int, err error) {
	m.send <- p
	return len(p), nil
}
