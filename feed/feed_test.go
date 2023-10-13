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

func TestTee(t *testing.T) {
	// sourceFeed -> sourceSub
	sourceFeed := feed.New[int]()
	sourceSub := sourceFeed.Subscribe()
	t.Cleanup(sourceSub.Unsubscribe)
	// nextFeed -> nextSub
	nextFeed := feed.New[int]()
	nextSub := nextFeed.Subscribe()
	t.Cleanup(nextSub.Unsubscribe)

	// sourceSub -> nextFeed
	feed.Tee(sourceSub, nextFeed)

	// nextSub receives values from sourceFeed and nextFeed.
	sourceFeed.Send(1)
	require.Equal(t, 1, <-nextSub.Recv())
	nextFeed.Send(2)
	require.Equal(t, 2, <-nextSub.Recv())
}
