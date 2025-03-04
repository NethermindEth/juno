package feed_test

import (
	"testing"

	"github.com/NethermindEth/juno/feed"
	"github.com/stretchr/testify/require"
)

func TestFeed(t *testing.T) {
	f := feed.New[int]()
	sub := f.Subscribe()
	subKeepLast := f.SubscribeKeepLast()

	f.Send(1)
	f.Send(2)
	require.Equal(t, 1, <-sub.Recv())
	require.Equal(t, 2, <-subKeepLast.Recv())
	select {
	case <-sub.Recv():
		// f should have skipped sub on the second send.
		require.Fail(t, "the channel should be empty")
	default:
	}
	select {
	case <-subKeepLast.Recv():
		// f should have skipped sub on the second send.
		require.Fail(t, "the channel should be empty")
	default:
	}
	f.Send(3)
	require.Equal(t, 3, <-sub.Recv())
	require.Equal(t, 3, <-subKeepLast.Recv())
	sub.Unsubscribe()
	_, ok := <-sub.Recv()
	require.False(t, ok, "channel should be closed")
	sub.Unsubscribe() // Unsubscribing twice is ok.

	subKeepLast.Unsubscribe()
	_, ok = <-subKeepLast.Recv()
	require.False(t, ok, "channel should be closed")
	subKeepLast.Unsubscribe() // Unsubscribing twice is ok.

	f.Send(1) // Sending without subscribers is ok.
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
	nextSubKeepLast := nextFeed.SubscribeKeepLast()
	t.Cleanup(nextSubKeepLast.Unsubscribe)

	// sourceSub -> nextFeed
	feed.Tee(sourceSub, nextFeed)

	// nextSub receives values from sourceFeed and nextFeed.
	sourceFeed.Send(1)
	require.Equal(t, 1, <-nextSub.Recv())
	require.Equal(t, 1, <-nextSubKeepLast.Recv())
	nextFeed.Send(2)
	require.Equal(t, 2, <-nextSub.Recv())
	require.Equal(t, 2, <-nextSubKeepLast.Recv())
}

func TestKeepLast(t *testing.T) {
	sourceFeed := feed.New[int]()
	keepEarlySub := sourceFeed.Subscribe()
	keepLastSub := sourceFeed.SubscribeKeepLast()

	for i := range 10 {
		sourceFeed.Send(i)
	}

	require.Equal(t, 0, <-keepEarlySub.Recv())
	require.Equal(t, 9, <-keepLastSub.Recv())
}
