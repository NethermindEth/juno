package jsonrpc

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
)

const (
	// http://man7.org/linux/man-pages/man7/unix.7.html
	maxIpcPathSize = int(108)
)

var (
	errPathTooLong = errors.New("path too long")
)

func createListener(endpoint string) (net.Listener, error) {
	var (
		l   net.Listener
		err error
	)
	// path + terminator
	if len(endpoint)+1 > maxIpcPathSize {
		return nil, errPathTooLong
	}
	err = preparePath(endpoint)
	if err != nil {
		return nil, err
	}
	l, err = net.Listen("unix", endpoint)
	if err != nil {
		return nil, err
	}
	return l, os.Chmod(endpoint, 0600)
}

func IpcDial(ctx context.Context, endpoint string) (net.Conn, error) {
	return new(net.Dialer).DialContext(ctx, "unix", endpoint)
}

func preparePath(path string) error {
	var err error
	if err := os.MkdirAll(filepath.Dir(path), 0751); err != nil {
		return err
	}
	err = os.Remove(path)
	if !os.IsNotExist(err) {
		return err
	}
	return nil
}
