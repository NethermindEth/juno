package broadcast_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/broadcaster/broadcast"
)

// benchmarkThroughput measures one configuration of Broadcast end to end: nPubs publishers
// into one ring, nSubs subscriptions draining channels. b.N sends are split across the
// publishers. Callers vary the parameters.
func benchmarkThroughput[T any](
	b *testing.B,
	payload T,
	label string,
	bufSize uint64,
	nPubs,
	nSubs int,
) {
	name := fmt.Sprintf("%s_buffer=%d/pubs=%d/subs=%d", label, bufSize, nPubs, nSubs)
	b.Run(name, func(b *testing.B) {
		bc := broadcast.New[T](bufSize)
		subbable := bc.NewSubscribable()

		type counts struct {
			recvd uint64
			lag   uint64
		}
		countCh := make(chan counts, nSubs)

		unsubs := make([]func(), nSubs)
		var readers sync.WaitGroup
		for i := range nSubs {
			sub := subbable.Subscribe()
			unsubs[i] = sub.Unsubscribe
			readers.Go(func() {
				var c counts
				for ev := range sub.Recv() {
					if _, ok := ev.AsEvent(); ok {
						c.recvd++
					} else {
						c.lag++
					}
				}
				countCh <- c
			})
		}

		// b.Loop cannot be used here: the iteration count has to be divided among the
		// publishers before any of them starts.
		perPub := b.N / nPubs
		var writers sync.WaitGroup
		b.ResetTimer()
		start := time.Now()
		for range nPubs {
			pub := bc.NewPublisher()
			writers.Go(func() {
				for range perPub {
					pub.Send(payload)
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

// BenchmarkBroadcastPublisherThroughput varies ring capacity and subscriber count for a
// single publisher, across four payload shapes.
func BenchmarkBroadcastPublisherThroughput(b *testing.B) {
	type BigStruct struct {
		Field1 [128]byte
		Field2 [256]int64
		Field3 string
		Field4 float64
	}
	payload := 10
	payloadBig := BigStruct{Field3: "benchmark big struct payload"}

	for _, bufSize := range []uint64{64, 256, 512, 1024, 2048, 4096} {
		for _, nSubs := range []int{1, 4, 32, 128, 256, 512, 1024} {
			benchmarkThroughput(b, payload, "int_value", bufSize, 1, nSubs)
			benchmarkThroughput(b, &payload, "int_ptr", bufSize, 1, nSubs)
			benchmarkThroughput(b, payloadBig, "big_struct_value", bufSize, 1, nSubs)
			benchmarkThroughput(b, &payloadBig, "big_struct_ptr", bufSize, 1, nSubs)
		}
	}
}

// BenchmarkBroadcastMultiPublisherThroughput holds capacity and payload fixed and varies
// publisher and subscriber counts instead.
func BenchmarkBroadcastMultiPublisherThroughput(b *testing.B) {
	payload := 10

	for _, nPubs := range []int{1, 2, 4, 8} {
		for _, nSubs := range []int{1, 32, 256, 1024} {
			benchmarkThroughput(b, &payload, "int_ptr", 1024, nPubs, nSubs)
		}
	}
}
