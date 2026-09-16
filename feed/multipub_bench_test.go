package feed_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/feed"
)

// BenchmarkFeedMultiPublisherThroughput is the feed twin of the broadcast package's
// multi-publisher benchmark: several goroutines call Send on one feed at once, with
// keep-last subscribers as in production; b.N sends are split across publishers.
func BenchmarkFeedMultiPublisherThroughput(b *testing.B) {
	payload := 10
	for _, nPubs := range []int{1, 2, 4, 8} {
		for _, nSubs := range []int{1, 32} {
			b.Run(fmt.Sprintf("pubs=%d/subs=%d", nPubs, nSubs), func(b *testing.B) {
				f := feed.New[*int]()

				unsubs := make([]func(), nSubs)
				var readers sync.WaitGroup
				readers.Add(nSubs)
				recvCh := make(chan uint64, nSubs)
				for i := range nSubs {
					sub := f.SubscribeKeepLast()
					unsubs[i] = sub.Unsubscribe
					go func() {
						defer readers.Done()
						recvd := uint64(0)
						for range sub.Recv() {
							recvd++
						}
						recvCh <- recvd
					}()
				}

				perPub := b.N / nPubs
				var writers sync.WaitGroup
				b.ResetTimer()
				start := time.Now()
				for range nPubs {
					writers.Go(func() {
						for range perPub {
							f.Send(&payload)
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
				close(recvCh)
				var totalRecvd uint64
				for r := range recvCh {
					totalRecvd += r
				}

				sent := float64(perPub * nPubs)
				b.ReportMetric(sent/duration, "msgs_sent_per_sec")
				b.ReportMetric(float64(totalRecvd)/duration, "msgs_recv_per_sec")
				b.ReportMetric(float64(totalRecvd)/float64(nSubs), "avg_msgs_recv_per_sub")
				b.ReportMetric(float64(totalRecvd)/(sent*float64(nSubs)), "delivered_fraction")
			})
		}
	}
}
