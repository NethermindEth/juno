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

// writeBufferSizes varies the ring capacity, and with it the writer's cache footprint.
var writeBufferSizes = []uint64{64, 256, 512, 1024, 2048, 4096}

func BenchmarkRingBufferWrite(b *testing.B) {
	msg := 42

	for _, bufSize := range writeBufferSizes {
		b.Run(fmt.Sprintf("buffer=%d", bufSize), func(b *testing.B) {
			rb := ring.NewRingBuffer[int](bufSize)

			for b.Loop() {
				rb.Write(msg)
			}
		})
	}
}

// BenchmarkRingBufferWriteConcurrent measures the cost of a write at 1 to 64 concurrent writers.
func BenchmarkRingBufferWriteConcurrent(b *testing.B) {
	const bufSize = 1024
	msg := 42

	for _, writers := range []int{1, 2, 4, 8, 16, 64} {
		b.Run(fmt.Sprintf("writers=%d", writers), func(b *testing.B) {
			rb := ring.NewRingBuffer[int](bufSize)

			perWriter := b.N / writers
			var wg sync.WaitGroup
			b.ResetTimer()
			for range writers {
				wg.Go(func() {
					for range perWriter {
						rb.Write(msg)
					}
				})
			}
			wg.Wait()
		})
	}
}

func BenchmarkEventOrLag_NewEvent(b *testing.B) {
	msg := 42
	for b.Loop() {
		_ = ring.NewEvent(msg)
	}
}

func BenchmarkEventOrLag_NewLag(b *testing.B) {
	seq := uint64(0)
	for b.Loop() {
		seq++
		_ = ring.NewLag[int](seq, seq+1)
	}
}

func BenchmarkEventOrLag_AsEvent(b *testing.B) {
	ev := ring.NewEvent(42)
	for b.Loop() {
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
				ctx := b.Context()

				iters := make([]iter.Seq[ring.EventOrLag[int]], nIters)
				for i := range iters {
					iters[i] = rb.Iterator(ctx)
				}

				for i := range numMsg {
					rb.Write(i)
				}

				var wg sync.WaitGroup

				b.ResetTimer()
				// All iterators start draining at once
				for _, iterSeq := range iters {
					wg.Go(func() {
						ctr := 0
						for ev := range iterSeq {
							if _, ok := ev.AsEvent(); ok {
								ctr++
								if ctr == numMsg {
									return
								}
							}
						}
					})
				}

				wg.Wait()
				b.StopTimer()
			})
		}
	}
}

// benchmarkRingBufferThroughput measures one configuration: nWriters writers against nIters
// concurrent iterators on a ring of bufSize. b.N writes are split across the writers.
// Callers vary the parameters.
func benchmarkRingBufferThroughput[T any](
	b *testing.B,
	payload T,
	label string,
	bufSize uint64,
	nWriters,
	nIters int,
) {
	name := fmt.Sprintf("%s_buffer=%d/writers=%d/iterators=%d", label, bufSize, nWriters, nIters)
	b.Run(name, func(b *testing.B) {
		rb := ring.NewRingBuffer[T](bufSize)
		ctx, cancel := context.WithCancel(b.Context())

		iters := make([]iter.Seq[ring.EventOrLag[T]], nIters)
		for i := range iters {
			iters[i] = rb.Iterator(ctx)
		}

		type counts struct {
			recvd uint64
			lag   uint64
		}
		countCh := make(chan counts, nIters)

		var wg sync.WaitGroup
		for _, iterSeq := range iters {
			wg.Go(func() {
				recvd := uint64(0)
				lag := uint64(0)
				for ev := range iterSeq {
					if _, ok := ev.AsEvent(); ok {
						recvd++
					} else {
						lag++
					}
				}

				countCh <- counts{recvd, lag}
			})
		}

		// b.Loop cannot be used here: the iteration count has to be divided among the
		// writers before any of them starts.
		perWriter := b.N / nWriters
		var writers sync.WaitGroup
		b.ResetTimer()
		start := time.Now()
		for range nWriters {
			writers.Go(func() {
				for range perWriter {
					rb.Write(payload)
				}
			})
		}
		writers.Wait()
		b.StopTimer()
		duration := time.Since(start).Seconds()

		cancel()
		wg.Wait()
		close(countCh)

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

		sent := float64(perWriter * nWriters)
		b.ReportMetric(sent/duration, "msgs_sent_per_sec")
		b.ReportMetric(float64(totalRecvd)/duration, "msgs_recv_per_sec")
		b.ReportMetric(avgRecvd, "avg_msgs_recv_per_iter")
		b.ReportMetric(avgLag, "avg_lag_per_iter")
		b.ReportMetric(float64(totalLag)/sent, "lag_per_msg")

		b.Logf("Sent: %.0f, Received (total): %d, Lag (total): %d, Duration: %.3fs",
			sent, totalRecvd, totalLag, duration)
		b.Logf("Received min/max per iterator: %d / %d", minRecvd, maxRecvd)
		b.Logf("Received avg per iterator: %.2f, Lag avg per iterator: %.2f", avgRecvd, avgLag)
	})
}

func BenchmarkRingBufferThroughput(b *testing.B) {
	// A large struct payload, to see what a heavy value copy costs per slot.
	type BigStruct struct {
		Field1 [128]byte  // 128 bytes
		Field2 [256]int64 // 256 * 8 = 2048 bytes
		Field3 string     // string header, pointer + len
		Field4 float64    // 8 bytes
	}
	payload := 10
	payloadBig := BigStruct{
		Field3: "benchmark big struct payload",
	}

	for _, bufSize := range []uint64{64, 256, 512, 1024, 2048, 4096} {
		for _, nIters := range []int{1, 4, 32, 128, 256, 512, 1024} {
			benchmarkRingBufferThroughput(b, payload, "int_value", bufSize, 1, nIters)
			benchmarkRingBufferThroughput(b, &payload, "int_ptr", bufSize, 1, nIters)
			benchmarkRingBufferThroughput(b, payloadBig, "big_struct_value", bufSize, 1, nIters)
			benchmarkRingBufferThroughput(b, &payloadBig, "big_struct_ptr", bufSize, 1, nIters)
		}
	}
}

// BenchmarkRingBufferMultiWriterThroughput holds capacity and payload fixed and varies
// writer and iterator counts instead.
func BenchmarkRingBufferMultiWriterThroughput(b *testing.B) {
	payload := 10

	for _, nWriters := range []int{1, 2, 4, 8} {
		for _, nIters := range []int{1, 32, 256, 1024} {
			benchmarkRingBufferThroughput(b, &payload, "int_ptr", 1024, nWriters, nIters)
		}
	}
}
