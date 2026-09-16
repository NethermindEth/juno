package ring_test

import (
	"context"
	"fmt"
	"iter"
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/broadcaster/broadcast/ring"
)

func BenchmarkRingBufferWrite(b *testing.B) {
	rb := ring.NewRingBuffer[int](1024)
	msg := 42

	b.ResetTimer()
	for range b.N {
		rb.Write(msg)
	}
}

func BenchmarkRingBufferWriteConcurrent(b *testing.B) {
	rb := ring.NewRingBuffer[int](1024)
	msg := 42

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rb.Write(msg)
		}
	})
}

func BenchmarkRingBufferIterator(b *testing.B) {
	rb := ring.NewRingBuffer[int](1024)
	ctx := context.Background()

	// Pre-fill buffer
	for i := range 1000 {
		msg := i
		rb.Write(msg)
	}

	b.ResetTimer()
	for range b.N {
		count := 0
		for ev := range rb.Iterator(ctx) {
			if ev.IsEvent() {
				count++
				if count >= 100 {
					break
				}
			}
		}
	}
}

func BenchmarkRingBufferIteratorMultipleReaders(b *testing.B) {
	rb := ring.NewRingBuffer[int](1024)
	ctx := context.Background()
	numReaders := []int{1, 4, 16, 64, 256}

	for _, n := range numReaders {
		b.Run(fmt.Sprintf("readers=%d", n), func(b *testing.B) {
			// Pre-fill buffer
			for i := range 1000 {
				msg := i
				rb.Write(msg)
			}

			b.ResetTimer()
			for range b.N {
				var wg sync.WaitGroup
				wg.Add(n)

				for range n {
					go func() {
						defer wg.Done()
						count := 0
						for ev := range rb.Iterator(ctx) {
							if ev.IsEvent() {
								count++
								if count >= 100 {
									break
								}
							}
						}
					}()
				}

				wg.Wait()
			}
		})
	}
}

// Disabled: waitForSequence is unexported. Slot-level micro-benchmark removed;
// BenchmarkRingBufferIterator exercises the same path via the public API.
/*
func BenchmarkSlotWaitForSequence(b *testing.B) {
	rb := ring.NewRingBuffer[int](4)
	ctx := context.Background()
	slot := rb.Slot(1)

	msg := 42
	rb.Write(msg)

	b.ResetTimer()
	for range b.N {
		_, _ = slot.WaitForSequence(ctx, 1)
	}
}
*/

func BenchmarkSlotWrite(b *testing.B) {
	rb := ring.NewRingBuffer[int](1024)
	msg := 42

	b.ResetTimer()
	for range b.N {
		rb.Write(msg)
	}
}

func BenchmarkEventOrLag_NewEvent(b *testing.B) {
	msg := 42
	b.ResetTimer()
	for range b.N {
		_ = ring.NewEvent(msg)
	}
}

func BenchmarkEventOrLag_NewLag(b *testing.B) {
	b.ResetTimer()
	for i := range b.N {
		_ = ring.NewLag[int](uint64(i), uint64(i+1))
	}
}

func BenchmarkEventOrLag_IsEvent(b *testing.B) {
	ev := ring.NewEvent(42)
	b.ResetTimer()
	for range b.N {
		_ = ev.IsEvent()
	}
}

func BenchmarkEventOrLag_AsEvent(b *testing.B) {
	ev := ring.NewEvent(42)
	b.ResetTimer()
	for range b.N {
		_, _ = ev.AsEvent()
	}
}

func BenchmarkRingBufferIteratorDrain_PreFill(b *testing.B) {
	numIterators := []int{1, 4, 32, 128, 256, 512, 1024}
	numMessages := []int{1024, 2048, 4096}

	for _, nIters := range numIterators {
		for _, numMsg := range numMessages {
			b.Run(fmt.Sprintf("iterators=%d, msg_count=%d", nIters, numMsg), func(b *testing.B) {
				rb := ring.NewRingBuffer[int](uint64(numMsg))
				ctx := context.Background()

				iters := make([]iter.Seq[ring.EventOrLag[int]], nIters)
				for i := range iters {
					iters[i] = rb.Iterator(ctx)
				}

				for i := range numMsg {
					msg := i
					rb.Write(msg)
				}

				var wg sync.WaitGroup
				wg.Add(len(iters))

				b.ResetTimer()
				// All iterators start draining at once
				for _, iterSeq := range iters {
					go func(seq iter.Seq[ring.EventOrLag[int]]) {
						defer wg.Done()
						ctr := 0
						for ev := range seq {
							if ev.IsEvent() {
								ctr++
								if ctr == numMsg {
									return
								}
							}
						}
					}(iterSeq)
				}

				wg.Wait()
				b.StopTimer()
			})
		}
	}
}

func benchmarkRingBufferThroughput[T any](
	b *testing.B,
	payload T,
	label string,
	bufferSizes []uint64,
) {
	for _, bufSize := range bufferSizes {
		b.Run(fmt.Sprintf("%s_buffer=%d", label, bufSize), func(b *testing.B) {
			numIterators := []int{1, 4, 32, 128, 256, 512, 1024}
			for _, nIters := range numIterators {
				b.Run(fmt.Sprintf("iterators=%d", nIters), func(b *testing.B) {
					rb := ring.NewRingBuffer[T](bufSize)
					ctx, cancel := context.WithCancel(context.Background())

					iters := make([]iter.Seq[ring.EventOrLag[T]], nIters)
					for i := range iters {
						iters[i] = rb.Iterator(ctx)
					}

					var wg sync.WaitGroup
					wg.Add(len(iters))

					type counts struct {
						recvd uint64
						lag   uint64
					}
					countCh := make(chan counts, nIters)

					for _, iterSeq := range iters {
						go func(seq iter.Seq[ring.EventOrLag[T]]) {
							defer wg.Done()
							recvd := uint64(0)
							lag := uint64(0)
							for ev := range seq {
								if ev.IsEvent() {
									recvd++
								} else if ev.IsLag() {
									lag++
								}
							}

							countCh <- counts{recvd, lag}
						}(iterSeq)
					}

					start := time.Now()
					for b.Loop() {
						rb.Write(payload)
					}

					cancel()
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

						if c.recvd < minRecvd {
							minRecvd = c.recvd
						}
						if c.recvd > maxRecvd {
							maxRecvd = c.recvd
						}
					}

					avgRecvd := float64(totalRecvd) / float64(nIters)
					avgLag := float64(totalLag) / float64(nIters)

					b.ReportMetric(float64(b.N)/duration, "msgs_sent_per_sec")
					b.ReportMetric(float64(totalRecvd)/duration, "msgs_recv_per_sec")
					b.ReportMetric(avgRecvd, "avg_msgs_recv_per_iter")
					b.ReportMetric(avgLag, "avg_lag_per_iter")
					b.ReportMetric(float64(totalLag)/float64(b.N), "lag_per_msg")

					b.Logf("%s buffer=%d iterators=%d Sent: %d, Received (total): %d, Lag (total): %d, "+
						"Duration: %.3fs", label, bufSize, nIters, b.N, totalRecvd, totalLag, duration)
					b.Logf("%s buffer=%d iterators=%d Received min/max per iterator: %d / %d",
						label, bufSize, nIters, minRecvd, maxRecvd)
					b.Logf("%s buffer=%d iterators=%d Received avg per iterator: %.2f, Lag avg per iterator: %.2f",
						label, bufSize, nIters, avgRecvd, avgLag)
				})
			}
		})
	}
}

func BenchmarkRingBufferThroughput(b *testing.B) {
	bufferSizes := []uint64{64, 256, 512, 1024, 2048, 4096}

	payload := 10
	// Run benchmark with int values as payload
	benchmarkRingBufferThroughput(b,
		payload,
		"int_value",
		bufferSizes,
	)

	// Run benchmark with pointers to int as payload
	benchmarkRingBufferThroughput(b,
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
	benchmarkRingBufferThroughput(b,
		payloadBig,
		"big_struct_value",
		bufferSizes,
	)

	benchmarkRingBufferThroughput(b,
		&payloadBig,
		"big_struct_ptr",
		bufferSizes,
	)
}
