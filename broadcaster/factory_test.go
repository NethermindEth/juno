package broadcaster_test

import (
	"testing"
	"time"

	"github.com/NethermindEth/juno/broadcaster"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/assert"
)

// TestNew_RoundTrip verifies that both Kind values produce a working hub:
// publishing through the Publisher reaches a Subscriber via the same channel
// shape regardless of the underlying implementation. The lag policy is supplied
// at NewSubscribable time, not at hub construction.
func TestNew_RoundTrip(t *testing.T) {
	cases := []struct {
		name      string
		opts      []broadcaster.Option
		lagPolicy broadcaster.LagPolicy[int]
	}{
		{
			name: "feed",
			opts: []broadcaster.Option{broadcaster.WithKind(broadcaster.KindFeed)},
		},
		{
			name: "broadcast_with_log_lag_policy",
			opts: []broadcaster.Option{
				broadcaster.WithKind(broadcaster.KindBroadcast),
				broadcaster.WithCapacity(16),
			},
			lagPolicy: broadcaster.LagPolicyLog[int](log.NewNopZapLogger()),
		},
		{
			name: "broadcast_with_drop_lag_policy",
			opts: []broadcaster.Option{
				broadcaster.WithKind(broadcaster.KindBroadcast),
				broadcaster.WithCapacity(16),
			},
			lagPolicy: broadcaster.LagPolicyDrop[int],
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hub := broadcaster.New[int](tc.opts...)

			pub := hub.NewPublisher()

			subbable := hub.NewSubscribable(tc.lagPolicy)

			sub := subbable.Subscribe()
			t.Cleanup(sub.Unsubscribe)

			want := 42
			pub.Send(want)

			select {
			case got := <-sub.Recv():
				assert.Equal(t, want, got)
			case <-time.After(time.Second):
				t.Fatal("timed out waiting for message")
			}
		})
	}
}

// TestNewSubscribable_PerConsumerPolicy is the property the design hinges on:
// two consumers ask the same hub for their own Subscribable, each with its own
// lag policy, and observe the stream through their own lens.
func TestNewSubscribable_PerConsumerPolicy(t *testing.T) {
	hub := broadcaster.New[int](
		broadcaster.WithKind(broadcaster.KindBroadcast), broadcaster.WithCapacity(16),
	)

	pub := hub.NewPublisher()

	subbableA := hub.NewSubscribable(broadcaster.LagPolicyDrop[int])
	subA := subbableA.Subscribe()
	t.Cleanup(subA.Unsubscribe)

	subbableB := hub.NewSubscribable(broadcaster.LagPolicyLog[int](log.NewNopZapLogger()))
	subB := subbableB.Subscribe()
	t.Cleanup(subB.Unsubscribe)

	want := 7
	pub.Send(want)

	for _, sub := range []broadcaster.Subscription[int]{subA, subB} {
		select {
		case got := <-sub.Recv():
			assert.Equal(t, want, got)
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for message")
		}
	}
}

// TestNew_UnknownKindPanics ensures the factory fails loudly on an unknown kind
// rather than returning a nil hub that would panic later in production.
func TestNew_UnknownKindPanics(t *testing.T) {
	assert.Panics(t, func() {
		broadcaster.New[int](broadcaster.WithKind(broadcaster.Kind(99)))
	})
}
