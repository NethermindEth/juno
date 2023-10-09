package feed_test

import (
	"testing"

	"github.com/NethermindEth/juno/feed"
	"github.com/stretchr/testify/require"
)

func TestFeed(t *testing.T) {
	f := feed.New[int]()
	sub := f.Subscribe()

	f.Send(1)
	f.Send(2)
	require.Equal(t, 1, <-sub.Recv())
	select {
	case <-sub.Recv():
		// f should have skipped sub on the second send.
		require.Fail(t, "the channel should be empty")
	default:
	}
	f.Send(2)
	require.Equal(t, 2, <-sub.Recv())
	sub.Unsubscribe()
	_, ok := <-sub.Recv()
	require.False(t, ok, "channel should be closed")
	sub.Unsubscribe() // Unsubscribing twice is ok.
	f.Send(1)         // Sending without subscribers is ok.
}
