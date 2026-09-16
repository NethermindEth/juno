package broadcaster_test

import (
	"testing"
	"time"

	"github.com/NethermindEth/juno/broadcaster"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// headEvent stands in for the kind of payload a real consumer (e.g. blockchain.l1HeadFeed)
// would publish through a BroadcastHub.
type headEvent struct {
	Number uint64
}

// TestEndToEnd_ConsumerPattern demonstrates the wiring a real consumer would do:
// pick a Kind from config, ask the hub for its own Subscribable bound to its lag
// policy, publish events, observe through Subscription. Sends are paced to one
// per receive so feed.Feed's keep-last drop semantics don't drop messages.
func TestEndToEnd_ConsumerPattern(t *testing.T) {
	cases := []struct {
		name      string
		opts      []broadcaster.Option
		lagPolicy broadcaster.LagPolicy[*headEvent]
	}{
		{
			name: "feed",
			opts: []broadcaster.Option{broadcaster.WithKind(broadcaster.KindFeed)},
		},
		{
			name: "broadcast_with_log_lag_policy",
			opts: []broadcaster.Option{
				broadcaster.WithKind(broadcaster.KindBroadcast),
				broadcaster.WithCapacity(64),
			},
			lagPolicy: broadcaster.LagPolicyLog[*headEvent](log.NewNopZapLogger()),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hub := broadcaster.New[*headEvent](tc.opts...)

			pub := hub.NewPublisher()

			subbable := hub.NewSubscribable(tc.lagPolicy)
			sub := subbable.Subscribe()
			defer sub.Unsubscribe()

			const N = 10
			for i := range uint64(N) {
				ev := &headEvent{Number: i}
				pub.Send(ev)

				select {
				case got := <-sub.Recv():
					assert.Equal(t, i, got.Number)
				case <-time.After(time.Second):
					t.Fatalf("timeout waiting for event %d", i)
				}
			}
		})
	}
}

// TestEndToEnd_BroadcastLagSurfaced confirms that when the publisher gets ahead
// of the subscriber by more than the ring's capacity, the consumer-supplied
// LagPolicyLog records the lag rather than silently dropping it.
func TestEndToEnd_BroadcastLagSurfaced(t *testing.T) {
	core, observed := observer.New(zapcore.WarnLevel)
	logger := log.NewZapLoggerWithCore(core)

	hub := broadcaster.New[int](
		broadcaster.WithKind(broadcaster.KindBroadcast), broadcaster.WithCapacity(4),
	)

	pub := hub.NewPublisher()

	subbable := hub.NewSubscribable(broadcaster.LagPolicyLog[int](logger))
	sub := subbable.Subscribe()
	defer sub.Unsubscribe()

	// Publish far past the ring capacity without draining; the subscriber must lag.
	for i := range 64 {
		v := i
		pub.Send(v)
	}

	// Drain whatever the subscriber sees with a short deadline to bound the test.
	deadline := time.After(500 * time.Millisecond)
drain:
	for {
		select {
		case <-sub.Recv():
		case <-deadline:
			break drain
		}
	}

	saw := false
	for _, e := range observed.All() {
		if e.Message == "broadcaster subscriber lagged" {
			ctx := e.ContextMap()
			assert.Contains(t, ctx, "missedSeq")
			assert.Contains(t, ctx, "nextSeq")
			saw = true
		}
	}
	assert.True(t, saw, "expected at least one lag warning to be logged")
}
