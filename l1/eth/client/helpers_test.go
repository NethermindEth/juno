package client_test

import (
	"testing"
	"time"

	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/l1/eth/client"
	"github.com/NethermindEth/juno/l1/internal/clienttest"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T, srv *clienttest.TestServer, opts ...client.Option) *client.Client {
	t.Helper()
	opts = append([]client.Option{client.WithLogger(log.NewNopZapLogger())}, opts...)
	cli, err := client.New(t.Context(), srv.WSURL(), opts...)
	require.NoError(t, err)
	t.Cleanup(cli.Close)
	// wait for server to register the ws conn
	require.Eventually(t, func() bool { return srv.WSConnCount() == 1 },
		time.Second, time.Millisecond, "server never registered the conn")
	return cli
}

func receiveLogs(t *testing.T, sink <-chan *eth.Log, n int, timeout time.Duration) []*eth.Log {
	t.Helper()
	deadline := time.After(timeout)
	out := make([]*eth.Log, 0, n)
	for len(out) < n {
		select {
		case log := <-sink:
			out = append(out, log)
		case <-deadline:
			return out
		}
	}
	return out
}
