package transport

import (
	"net/http"
	"time"
)

const (
	sendPingTimeout = 10 * time.Second
	pingTimeout     = 5 * time.Second
)

// New returns an HTTP transport that closes an HTTP/2 connection when the peer stops
// answering pings.
func New() *http.Transport {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	// Go reuses an HTTP/2 connection after a request timeout. Without pings, a connection
	// that a network change made silent (for example, a WAN failover) stays in use for minutes.
	tr.HTTP2 = &http.HTTP2Config{
		SendPingTimeout: sendPingTimeout,
		PingTimeout:     pingTimeout,
	}
	return tr
}
