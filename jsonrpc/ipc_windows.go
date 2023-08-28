//go:build windows
// +build windows

package jsonrpc

import (
	"context"
	"errors"
	"net"
)

// TODO: windows pipe requires another dependency github.com/Microsoft/go-winio. Should be pulled?
var errNotSupported = errors.New("ipc: windows not supported")

func createListener(endpoint string) (net.Listener, error) {
	return nil, errNotSupported
}

func IpcDial(ctx context.Context, endpoint string) (net.Conn, error) {
	return nil, errNotSupported
}
