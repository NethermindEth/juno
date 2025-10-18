package broadcast_test

import (
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/broadcast"
	"github.com/stretchr/testify/require"
)

func BenchmarkBroadcastReceiverDrain_PreFill(b *testing.B) {
	numSubscribers := []int{1, 4, 32, 128, 256, 512, 1024}
	numMessages := []int{1024, 2048, 4096}

	for _, nSubs := range numSubscribers {
		for _, numMsg := range numMessages {
			b.Run("subs="+strconv.Itoa(nSubs)+", msg_count="+strconv.Itoa(numMsg), func(b *testing.B) {
				bc := broadcast.New[int](uint64(numMsg))

				subs := make([]*broadcast.Subscription[int], nSubs)
				for i := range subs {
					subs[i] = bc.Subscribe()
				}

				for i := range numMsg {
					require.NoError(b, bc.Send(&i))
				}
				// Close broadcast. Drain-on-close semantics.
				bc.Close()
				var wg sync.WaitGroup
				wg.Add(len(subs))

				b.ResetTimer()
				// All subscribers start draining at once
				for _, sub := range subs {
					go func(sub *broadcast.Subscription[int]) {
						defer wg.Done()
						ctr := 0
						out := sub.Recv()
						for {
							_, ok := <-out
							if !ok {
								return
							}
							ctr++

							if ctr == numMsg {
								return
							}
						}
					}(sub)
				}

				wg.Wait()
				b.StopTimer()
			})
		}
	}
}

func BenchmarkSubsribeUnsubscribe(b *testing.B) {
	bc := broadcast.New[int](1024)
	defer bc.Close()

	for b.Loop() {
		sub := bc.Subscribe()
		sub.Unsubscribe()
	}
}

func BenchmarkBroadcastSenderThroughput_Small(b *testing.B) {
	bc := broadcast.New[int](1024)

	const nSubs = 32
	subs := make([]*broadcast.Subscription[int], nSubs)
	for i := range nSubs {
		subs[i] = bc.Subscribe()
	}

	var wg sync.WaitGroup
	wg.Add(nSubs)
	for _, sub := range subs {
		go func(s *broadcast.Subscription[int]) {
			defer wg.Done()
			for range s.Recv() {
				// drain
			}
		}(sub)
	}

	b.ResetTimer()
	for i := range b.N {
		if err := bc.Send(&i); err != nil {
			if err == broadcast.ErrClosed {
				break
			}
			b.Fatal(err)
		}
	}
	b.StopTimer()

	bc.Close()
	wg.Wait()
}

func benchmarkBroadcastSenderThroughput[T any](
	b *testing.B,
	payload T,
	label string,
	bufferSizes []uint64,
) {
	for _, bufSize := range bufferSizes {
		b.Run(fmt.Sprintf("%s_buffer=%d", label, bufSize), func(b *testing.B) {
			numSubscribers := []int{1, 4, 32, 128, 256, 512, 1024}
			for _, nSubs := range numSubscribers {
				b.Run(fmt.Sprintf("subs=%d", nSubs), func(b *testing.B) {
					bc := broadcast.New[T](bufSize)

					subs := make([]*broadcast.Subscription[T], nSubs)
					for i := range subs {
						subs[i] = bc.Subscribe()
					}

					var wg sync.WaitGroup
					wg.Add(len(subs))

					type counts struct {
						recvd uint64
						lag   uint64
					}
					countCh := make(chan counts, nSubs)

					for _, sub := range subs {
						go func(sub *broadcast.Subscription[T]) {
							defer wg.Done()
							recvd := uint64(0)
							lag := uint64(0)
							out := sub.Recv()
							for ev := range out {
								if ev.IsEvent() {
									recvd++
								} else if ev.IsLag() {
									lag++
								}
							}

							countCh <- counts{recvd, lag}
						}(sub)
					}

					start := time.Now()
					for b.Loop() {
						if err := bc.Send(&payload); err != nil {
							b.Fatal(err)
						}
					}

					bc.Close()
					wg.Wait()
					close(countCh)

					duration := time.Since(start).Seconds()

					var totalRecvd uint64
					var totalLag uint64
					minRecvd := ^uint64(0) // max uint64
					maxRecvd := uint64(0)

					for c := range countCh {
						totalRecvd += c.recvd
						totalLag += c.lag

						minRecvd = min(minRecvd, c.recvd)
						maxRecvd = max(maxRecvd, c.recvd)
					}

					avgRecvd := float64(totalRecvd) / float64(nSubs)
					avgLag := float64(totalLag) / float64(nSubs)

					b.ReportMetric(float64(b.N)/duration, "msgs_sent_per_sec")
					b.ReportMetric(float64(totalRecvd)/duration, "msgs_recv_per_sec")
					b.ReportMetric(avgRecvd, "avg_msgs_recv_per_sub")
					b.ReportMetric(avgLag, "avg_lag_per_sub")
					b.ReportMetric(float64(totalLag)/float64(b.N), "lag_per_msg")

					b.Logf("%s buffer=%d subs=%d Sent: %d, Received (total): %d, Lag (total): %d, Duration: %.3fs",
						label, bufSize, nSubs, b.N, totalRecvd, totalLag, duration)
					b.Logf("%s buffer=%d subs=%d Received min/max per subscriber: %d / %d",
						label, bufSize, nSubs, minRecvd, maxRecvd)
					b.Logf("%s buffer=%d subs=%d Received avg per subscriber: %.2f, Lag avg per subscriber: %.2f",
						label, bufSize, nSubs, avgRecvd, avgLag)
				})
			}
		})
	}
}

func BenchmarkBroadcastPublisherThroughput(b *testing.B) {
	bufferSizes := []uint64{64, 256, 512, 1024, 2048, 4096}

	payload := 10
	// Run benchmark with int values as payload
	benchmarkBroadcastSenderThroughput(b,
		payload,
		"int_value",
		bufferSizes,
	)

	// Run benchmark with pointers to int as payload
	benchmarkBroadcastSenderThroughput(b,
		&payload,
		"int_ptr",
		bufferSizes,
	)

	// Benchmark with a large struct payload to simulate heavy payloads
	type BigStruct struct {
		Field1 [128]byte  // 128 bytes
		Field2 [256]int64 // 256 * 8 = 2048 bytes
		Field3 string     // string header, pointer + len
		Field4 float64    // 8 bytes
	}
	payloadBig := BigStruct{
		Field3: "benchmark big struct payload",
	}
	benchmarkBroadcastSenderThroughput(b,
		payloadBig,
		"big_struct_value",
		bufferSizes,
	)

	benchmarkBroadcastSenderThroughput(b,
		&payloadBig,
		"big_struct_ptr",
		bufferSizes,
	)
}
