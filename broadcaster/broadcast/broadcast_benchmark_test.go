package broadcast_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/broadcaster/broadcast"
)

// benchmarkPublisherThroughput mirrors utils/broadcast's sender throughput benchmark
// at the Broadcast level: ring + IterToChan, EventOrLag on a channel, no lag policy.
func benchmarkPublisherThroughput[T any](
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
					pub := bc.NewPublisher()
					subbable := bc.NewSubscribable()

					unsubs := make([]func(), nSubs)
					var wg sync.WaitGroup
					wg.Add(nSubs)

					type counts struct {
						recvd uint64
						lag   uint64
					}
					countCh := make(chan counts, nSubs)

					for i := range nSubs {
						sub := subbable.Subscribe()
						unsubs[i] = sub.Unsubscribe
						go func() {
							defer wg.Done()
							recvd := uint64(0)
							lag := uint64(0)
							for ev := range sub.Recv() {
								if ev.IsEvent() {
									recvd++
								} else if ev.IsLag() {
									lag++
								}
							}
							countCh <- counts{recvd, lag}
						}()
					}

					start := time.Now()
					for b.Loop() {
						pub.Send(payload)
					}

					for _, unsub := range unsubs {
						unsub()
					}
					wg.Wait()
					close(countCh)

					duration := time.Since(start).Seconds()

					var totalRecvd, totalLag uint64
					for c := range countCh {
						totalRecvd += c.recvd
						totalLag += c.lag
					}
					avgRecvd := float64(totalRecvd) / float64(nSubs)
					avgLag := float64(totalLag) / float64(nSubs)

					b.ReportMetric(float64(b.N)/duration, "msgs_sent_per_sec")
					b.ReportMetric(float64(totalRecvd)/duration, "msgs_recv_per_sec")
					b.ReportMetric(avgRecvd, "avg_msgs_recv_per_sub")
					b.ReportMetric(avgLag, "avg_lag_per_sub")
					b.ReportMetric(float64(totalLag)/float64(b.N), "lag_per_msg")
				})
			}
		})
	}
}

func BenchmarkBroadcastPublisherThroughput(b *testing.B) {
	bufferSizes := []uint64{64, 256, 512, 1024, 2048, 4096}

	payload := 10
	benchmarkPublisherThroughput(b, payload, "int_value", bufferSizes)
	benchmarkPublisherThroughput(b, &payload, "int_ptr", bufferSizes)

	type BigStruct struct {
		Field1 [128]byte
		Field2 [256]int64
		Field3 string
		Field4 float64
	}
	payloadBig := BigStruct{Field3: "benchmark big struct payload"}
	benchmarkPublisherThroughput(b, payloadBig, "big_struct_value", bufferSizes)
	benchmarkPublisherThroughput(b, &payloadBig, "big_struct_ptr", bufferSizes)
}

// BenchmarkBroadcastMultiPublisherThroughput has several publishers sending into one
// ring at once, pointer payload, capacity 1024; b.N sends are split across publishers.
func BenchmarkBroadcastMultiPublisherThroughput(b *testing.B) {
	payload := 10
	const bufSize = 1024
	for _, nPubs := range []int{1, 2, 4, 8} {
		for _, nSubs := range []int{1, 32} {
			b.Run(fmt.Sprintf("pubs=%d/subs=%d", nPubs, nSubs), func(b *testing.B) {
				bc := broadcast.New[*int](bufSize)
				subbable := bc.NewSubscribable()

				unsubs := make([]func(), nSubs)
				var readers sync.WaitGroup
				readers.Add(nSubs)
				type counts struct {
					recvd uint64
					lag   uint64
				}
				countCh := make(chan counts, nSubs)
				for i := range nSubs {
					sub := subbable.Subscribe()
					unsubs[i] = sub.Unsubscribe
					go func() {
						defer readers.Done()
						var c counts
						for ev := range sub.Recv() {
							if ev.IsEvent() {
								c.recvd++
							} else if ev.IsLag() {
								c.lag++
							}
						}
						countCh <- c
					}()
				}

				perPub := b.N / nPubs
				var writers sync.WaitGroup
				b.ResetTimer()
				start := time.Now()
				for range nPubs {
					pub := bc.NewPublisher()
					writers.Go(func() {
						for range perPub {
							pub.Send(&payload)
						}
					})
				}
				writers.Wait()
				b.StopTimer()
				duration := time.Since(start).Seconds()

				for _, unsub := range unsubs {
					unsub()
				}
				readers.Wait()
				close(countCh)
				var totalRecvd, totalLag uint64
				for c := range countCh {
					totalRecvd += c.recvd
					totalLag += c.lag
				}

				sent := float64(perPub * nPubs)
				b.ReportMetric(sent/duration, "msgs_sent_per_sec")
				b.ReportMetric(float64(totalRecvd)/duration, "msgs_recv_per_sec")
				b.ReportMetric(float64(totalRecvd)/float64(nSubs), "avg_msgs_recv_per_sub")
				b.ReportMetric(float64(totalLag)/float64(nSubs), "avg_lag_per_sub")
				b.ReportMetric(float64(totalRecvd)/(sent*float64(nSubs)), "delivered_fraction")
			})
		}
	}
}
