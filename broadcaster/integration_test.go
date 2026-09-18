package broadcaster_test

import (
	"testing"
	"testing/synctest"
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

			const numEvents = 10
			for i := range uint64(numEvents) {
				pub.Send(&headEvent{Number: i})

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
	synctest.Test(t, func(t *testing.T) {
		loggerCore, observed := observer.New(zapcore.WarnLevel)
		logger := log.NewZapLoggerWithCore(loggerCore)

		hub := broadcaster.New[int](
			broadcaster.WithKind(broadcaster.KindBroadcast), broadcaster.WithCapacity(4),
		)

		pub := hub.NewPublisher()

		subbable := hub.NewSubscribable(broadcaster.LagPolicyLog[int](logger))
		sub := subbable.Subscribe()
		defer sub.Unsubscribe()

		// Publish far past the ring capacity without draining; the subscriber must lag.
		for i := range 64 {
			pub.Send(i)
		}

		// Drain until the subscriber has nothing left to hand over. It may have been
		// scheduled before the sends and be parked on a full channel, so receiving is what
		// lets it reach its overwritten sequence and report the lag.
	drain:
		for {
			synctest.Wait()
			select {
			case <-sub.Recv():
			default:
				break drain
			}
		}

		saw := false
		for _, entry := range observed.All() {
			if entry.Message == "broadcaster subscriber lagged" {
				context := entry.ContextMap()
				assert.Contains(t, context, "missedSeq")
				assert.Contains(t, context, "nextSeq")
				saw = true
			}
		}
		assert.True(t, saw, "expected at least one lag warning to be logged")
	})
}
