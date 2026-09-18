package broadcaster_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/broadcaster"
)

// hubVariant pairs a human-readable name with the options the factory needs.
// Feed is one variant (no capacity); broadcast spans the buffer-size axis.
type hubVariant struct {
	name string
	opts []broadcaster.Option
}

func defaultVariants() []hubVariant {
	return append([]hubVariant{
		{"feed", []broadcaster.Option{broadcaster.WithKind(broadcaster.KindFeed)}},
	}, broadcastVariants([]uint64{64, 256, 512, 1024, 2048, 4096})...)
}

func broadcastVariants(bufferSizes []uint64) []hubVariant {
	out := make([]hubVariant, 0, len(bufferSizes))
	for _, sz := range bufferSizes {
		out = append(out, hubVariant{
			name: fmt.Sprintf("broadcast_cap=%d", sz),
			opts: []broadcaster.Option{
				broadcaster.WithKind(broadcaster.KindBroadcast),
				broadcaster.WithCapacity(sz),
			},
		})
	}
	return out
}

func BenchmarkHubSend(b *testing.B) {
	for _, v := range defaultVariants() {
		b.Run(v.name, func(b *testing.B) {
			hub := broadcaster.New[int](v.opts...)
			pub := hub.NewPublisher()

			const nSubs = 32
			subbable := hub.NewSubscribable(broadcaster.LagPolicyDrop[int])
			subs := make([]broadcaster.Subscription[int], nSubs)
			for i := range subs {
				s := subbable.Subscribe()
				subs[i] = s
			}

			var wg sync.WaitGroup
			for _, s := range subs {
				wg.Go(func() {
					for range s.Recv() {
					}
				})
			}

			msg := 42
			b.ResetTimer()
			for b.Loop() {
				pub.Send(msg)
			}
			b.StopTimer()

			for _, s := range subs {
				s.Unsubscribe()
			}
			wg.Wait()
		})
	}
}

func BenchmarkHubSubscribeUnsubscribe(b *testing.B) {
	for _, v := range defaultVariants() {
		b.Run(v.name, func(b *testing.B) {
			hub := broadcaster.New[int](v.opts...)

			subbable := hub.NewSubscribable(broadcaster.LagPolicyDrop[int])

			b.ResetTimer()
			for b.Loop() {
				s := subbable.Subscribe()
				s.Unsubscribe()
			}
		})
	}
}

// BenchmarkHubReceiverDrain_PreFill writes msg_count messages, then starts the
// subscribers and times draining the backlog. Broadcast-only: feed's KeepLast
// channel is size 1 and Send drops when the chan is full, so a pre-fill phase
// without an active drainer measures dropping, not throughput.
func BenchmarkHubReceiverDrain_PreFill(b *testing.B) {
	numSubscribers := []int{1, 4, 32, 128, 256, 512, 1024}
	numMessages := []int{1024, 2048, 4096}

	for _, nSubs := range numSubscribers {
		for _, numMsg := range numMessages {
			b.Run(fmt.Sprintf("subs=%d, msg_count=%d", nSubs, numMsg), func(b *testing.B) {
				hub := broadcaster.New[int](
					broadcaster.WithKind(broadcaster.KindBroadcast),
					broadcaster.WithCapacity(uint64(numMsg)),
				)
				pub := hub.NewPublisher()

				subbable := hub.NewSubscribable(broadcaster.LagPolicyDrop[int])
				subs := make([]broadcaster.Subscription[int], nSubs)
				for i := range subs {
					s := subbable.Subscribe()
					subs[i] = s
				}

				for i := range numMsg {
					msg := i
					pub.Send(msg)
				}

				var wg sync.WaitGroup

				b.ResetTimer()
				for _, s := range subs {
					wg.Go(func() {
						received := 0
						for range s.Recv() {
							received++
							if received == numMsg {
								return
							}
						}
					})
				}

				wg.Wait()
				b.StopTimer()

				for _, s := range subs {
					s.Unsubscribe()
				}
			})
		}
	}
}

func benchmarkHubPublisherThroughput[T any](
	b *testing.B,
	payload T,
	label string,
	variants []hubVariant,
) {
	for _, v := range variants {
		b.Run(fmt.Sprintf("%s/%s", label, v.name), func(b *testing.B) {
			numSubscribers := []int{1, 4, 32, 128, 256, 512, 1024}
			for _, nSubs := range numSubscribers {
				b.Run(fmt.Sprintf("subs=%d", nSubs), func(b *testing.B) {
					hub := broadcaster.New[T](v.opts...)
					pub := hub.NewPublisher()

					subbable := hub.NewSubscribable(broadcaster.LagPolicyDrop[T])
					subs := make([]broadcaster.Subscription[T], nSubs)
					for i := range subs {
						s := subbable.Subscribe()
						subs[i] = s
					}

					var wg sync.WaitGroup

					perSubRecvd := make([]uint64, nSubs)
					for i, s := range subs {
						wg.Go(func() {
							recvd := uint64(0)
							for range s.Recv() {
								recvd++
							}
							perSubRecvd[i] = recvd
						})
					}

					start := time.Now()
					b.ResetTimer()
					for b.Loop() {
						pub.Send(payload)
					}
					b.StopTimer()

					// Closing the last publisher cancels broadcast subscribers.
					for _, s := range subs {
						s.Unsubscribe()
					}
					wg.Wait()

					duration := time.Since(start).Seconds()

					var totalRecvd uint64
					minRecvd := ^uint64(0)
					maxRecvd := uint64(0)
					for _, r := range perSubRecvd {
						totalRecvd += r
						minRecvd = min(minRecvd, r)
						maxRecvd = max(maxRecvd, r)
					}

					sent := uint64(b.N) //nolint:gosec // b.N is non-negative
					avgRecvd := float64(totalRecvd) / float64(nSubs)

					// Derive per-sub drops as sent - recvd for both kinds, so feed
					// and broadcast use the same accounting.
					var missed uint64
					for _, r := range perSubRecvd {
						if r < sent {
							missed += sent - r
						}
					}
					avgMissed := float64(missed) / float64(nSubs)

					deliveredFraction := 0.0
					if sent > 0 {
						deliveredFraction = float64(totalRecvd) / float64(sent*uint64(nSubs))
					}

					b.ReportMetric(float64(sent)/duration, "msgs_sent_per_sec")
					b.ReportMetric(float64(totalRecvd)/duration, "msgs_recv_per_sec")
					b.ReportMetric(avgRecvd, "avg_msgs_recv_per_sub")
					b.ReportMetric(avgMissed, "avg_missed_per_sub")
					b.ReportMetric(deliveredFraction, "delivered_fraction")

					b.Logf("%s %s subs=%d sent=%d recv_total=%d missed_total=%d duration=%.3fs",
						label, v.name, nSubs, sent, totalRecvd, missed, duration)
					b.Logf("%s %s subs=%d recv_min/max_per_sub=%d/%d avg_recv_per_sub=%.2f "+
						"avg_missed_per_sub=%.2f", label, v.name, nSubs, minRecvd, maxRecvd, avgRecvd, avgMissed)
				})
			}
		})
	}
}

func BenchmarkHubPublisherThroughput(b *testing.B) {
	variants := defaultVariants()

	payload := 10
	benchmarkHubPublisherThroughput(b, payload, "int_value", variants)
	benchmarkHubPublisherThroughput(b, &payload, "int_ptr", variants)

	type BigStruct struct {
		Field1 [128]byte
		Field2 [256]int64
		Field3 string
		Field4 float64
	}
	payloadBig := BigStruct{
		Field3: "benchmark big struct payload",
	}
	benchmarkHubPublisherThroughput(b, payloadBig, "big_struct_value", variants)
	benchmarkHubPublisherThroughput(b, &payloadBig, "big_struct_ptr", variants)
}
