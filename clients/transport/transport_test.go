package transport_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/NethermindEth/juno/clients/transport"
	"github.com/stretchr/testify/require"
)

// pipeNetwork drops all bytes on existing connections after failover, as a WAN failover does.
type pipeNetwork struct {
	accepted chan net.Conn
	closed   chan struct{}
	close    sync.Once
	mu       sync.Mutex
	links    []*link
	dials    atomic.Int64
}

type link struct {
	conns [4]net.Conn
	dead  atomic.Bool
}

func newPipeNetwork() *pipeNetwork {
	return &pipeNetwork{accepted: make(chan net.Conn), closed: make(chan struct{})}
}

func (n *pipeNetwork) Accept() (net.Conn, error) {
	select {
	case c := <-n.accepted:
		return c, nil
	case <-n.closed:
		return nil, net.ErrClosed
	}
}

func (n *pipeNetwork) Close() error {
	n.close.Do(func() { close(n.closed) })
	return nil
}

func (n *pipeNetwork) Addr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 443}
}

func (n *pipeNetwork) DialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	client, clientRelay := net.Pipe()
	serverRelay, server := net.Pipe()
	l := &link{conns: [4]net.Conn{client, clientRelay, serverRelay, server}}

	select {
	case n.accepted <- server:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	n.dials.Add(1)
	n.mu.Lock()
	n.links = append(n.links, l)
	n.mu.Unlock()

	go l.relay(serverRelay, clientRelay)
	go l.relay(clientRelay, serverRelay)
	return client, nil
}

func (l *link) relay(dst, src net.Conn) {
	buf := make([]byte, 32<<10)
	for {
		n, err := src.Read(buf)
		if err != nil {
			return
		}
		if l.dead.Load() {
			continue
		}
		if _, err := dst.Write(buf[:n]); err != nil {
			return
		}
	}
}

func (n *pipeNetwork) failover() {
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, l := range n.links {
		l.dead.Store(true)
	}
}

func (n *pipeNetwork) shutdown() {
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, l := range n.links {
		for _, c := range l.conns {
			c.Close()
		}
	}
}

func TestNewReplacesSilentHTTP2Connection(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		network := newPipeNetwork()
		srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, r.Proto)
		}))
		require.NoError(t, srv.Listener.Close())
		srv.Listener = network
		srv.EnableHTTP2 = true
		srv.StartTLS()

		rootCAs := x509.NewCertPool()
		rootCAs.AddCert(srv.Certificate())
		tr := transport.New()
		tr.Proxy = nil
		tr.DialContext = network.DialContext
		tr.TLSClientConfig = &tls.Config{RootCAs: rootCAs}
		client := &http.Client{Transport: tr, Timeout: 5 * time.Second}
		t.Cleanup(func() {
			tr.CloseIdleConnections()
			network.shutdown()
			srv.Close()
		})

		get := func() (string, error) {
			req, err := http.NewRequestWithContext(
				t.Context(), http.MethodGet, "https://example.com/", http.NoBody,
			)
			if err != nil {
				return "", err
			}
			res, err := client.Do(req)
			if err != nil {
				return "", err
			}
			defer res.Body.Close()
			body, err := io.ReadAll(res.Body)
			return string(body), err
		}

		proto, err := get()
		require.NoError(t, err)
		require.Equal(t, "HTTP/2.0", proto)

		network.failover()
		start := time.Now()
		for {
			if _, err = get(); err == nil {
				break
			}
			require.Less(t, time.Since(start), time.Minute, "last request: %v", err)
		}
		require.Equal(t, int64(2), network.dials.Load())
	})
}
