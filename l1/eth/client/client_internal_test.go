package client

import (
	"context"
	"testing"
	"time"

	"github.com/NethermindEth/juno/l1/internal/clienttest"
	"github.com/stretchr/testify/require"
)

func TestClient_RedialHonoursCtxWhileDialInFlight(t *testing.T) {
	srv := clienttest.NewTestServer(t)
	cli, err := New(t.Context(), srv.WSURL())
	require.NoError(t, err)
	t.Cleanup(cli.Close)

	started := make(chan struct{})
	gate := make(chan struct{})
	t.Cleanup(func() { close(gate) })
	go func() {
		_, _, _ = cli.dials.Do("dial", func() (any, error) {
			close(started)
			<-gate // a dial that never finishes
			return nil, nil
		})
	}()
	<-started // the hanging flight owns the key before the real caller joins

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	done := make(chan error, 1)
	go func() {
		_, err := cli.redial(ctx, cli.tr)
		done <- err
	}()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("redial ignored ctx cancellation while a dial was in flight")
	}
}
