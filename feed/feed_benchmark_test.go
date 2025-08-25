package feed_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/feed"
)

func BenchmarkSubsribe(b *testing.B) {
	bc := feed.New[int]()

	for b.Loop() {
		sub := bc.Subscribe()
		sub.Unsubscribe()
	}
}

func benchmarkBroadcastSenderThroughput[T any](
	b *testing.B,
	payload T,
	label string,
) {
	numSubscribers := []int{1, 4, 32, 128, 256, 512, 1024}
	for _, nSubs := range numSubscribers {
		b.Run(fmt.Sprintf("subs=%d", nSubs), func(b *testing.B) {
			bc := feed.New[T]()

			subs := make([]*feed.Subscription[T], nSubs)
			for i := range subs {
				subs[i] = bc.Subscribe()
			}

			var wg sync.WaitGroup
			wg.Add(len(subs))

			type counts struct {
				recvd uint64
			}
			countCh := make(chan counts, nSubs)

			readerCtx, cancelReader := context.WithCancel(b.Context())

			for _, sub := range subs {
				go func(sub *feed.Subscription[T]) {
					defer wg.Done()
					recvd := uint64(0)
					out := sub.Recv()
					for {
						select {
						case <-out:
							recvd++
						case <-readerCtx.Done():
							countCh <- counts{recvd}
							return
						}
					}
				}(sub)
			}

			start := time.Now()
			for b.Loop() {
				bc.Send(payload)
			}

			cancelReader()
			wg.Wait()
			close(countCh)

			duration := time.Since(start).Seconds()

			var totalRecvd uint64
			minRecvd := ^uint64(0) // max uint64
			maxRecvd := uint64(0)

			for c := range countCh {
				totalRecvd += c.recvd

				minRecvd = min(minRecvd, c.recvd)
				maxRecvd = max(maxRecvd, c.recvd)
			}

			avgRecvd := float64(totalRecvd) / float64(nSubs)

			b.ReportMetric(float64(b.N)/duration, "msgs_sent_per_sec")
			b.ReportMetric(float64(totalRecvd)/duration, "msgs_recv_per_sec")
			b.ReportMetric(avgRecvd, "avg_msgs_recv_per_sub")

			b.Logf("%s subs=%d Sent: %d, Received (total): %d, Duration: %.3fs",
				label, nSubs, b.N, totalRecvd, duration)
			b.Logf("%s subs=%d Received min/max per subscriber: %d / %d",
				label, nSubs, minRecvd, maxRecvd)
			b.Logf("%s subs=%d Received avg per subscriber: %.2f",
				label, nSubs, avgRecvd)
		})
	}
}

func BenchmarkBroadcastPublisherThroughput(b *testing.B) {
	payload := 10
	// Run benchmark with int values as payload
	benchmarkBroadcastSenderThroughput(b,
		payload,
		"int_value",
	)

	// Run benchmark with pointers to int as payload
	benchmarkBroadcastSenderThroughput(b,
		&payload,
		"int_ptr",
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
	)

	benchmarkBroadcastSenderThroughput(b,
		&payloadBig,
		"big_struct_ptr",
	)
}
