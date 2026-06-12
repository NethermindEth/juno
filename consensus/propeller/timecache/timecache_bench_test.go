package timecache_test

import (
	"math/rand/v2"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NethermindEth/juno/consensus/propeller/timecache"
)

func BenchmarkTimeCacheAdd(b *testing.B) {
	b.Run("small cache size", func(b *testing.B) {
		tc := timecache.New[int](100, 3*time.Second)
		for i := range b.N {
			tc.Add(&i)
		}
	})

	b.Run("big cache size", func(b *testing.B) {
		tc := timecache.New[int](5000, 3*time.Second)
		for i := range b.N {
			tc.Add(&i)
		}
	})

	b.Run("big cache size with some expired values", func(b *testing.B) {
		const size = 2000
		tc := timecache.New[int](size, 2*time.Second)
		for i := range size - 1 {
			tc.Add(&i)
			time.Sleep(1 * time.Millisecond)
		}

		// Let some values expire
		time.Sleep(500 * time.Millisecond)
		b.ResetTimer()

		// Add a lot of new ones
		for i := range b.N {
			tc.Add(&i)
		}
	})

	b.Run("custom key", func(b *testing.B) {
		// the same size as `messageKey`
		type key [10]uint64

		tc := timecache.New[key](5000, 3*time.Second)
		for i := range b.N {
			key := key{}
			key[0] = uint64(i)
			tc.Add(&key)
		}
	})
}

func BenchmarkTimeCacheGet(b *testing.B) {
	b.Run("empty cache", func(b *testing.B) {
		tc := timecache.New[int](100, 3*time.Second)
		for i := range b.N {
			tc.Get(&i)
		}
	})

	b.Run("full unexpired cache", func(b *testing.B) {
		const size = 10000
		tc := timecache.New[int](size, 1*time.Hour)
		for i := range size {
			tc.Add(&i)
		}
		b.ResetTimer()

		for i := range b.N {
			key := i % size
			tc.Get(&key)
		}
	})

	b.Run("full with some expired values", func(b *testing.B) {
		const size = 2000
		tc := timecache.New[int](size, 2*time.Second)
		for i := range size / 2 {
			tc.Add(&i)
			time.Sleep(time.Millisecond)
		}
		time.Sleep(200 * time.Millisecond)

		b.ResetTimer()

		for i := range b.N {
			key := i % size
			tc.Get(&key)
		}
	})

	b.Run("while additions are ocurring at the same time", func(b *testing.B) {
		const size = 100
		tc := timecache.New[int64](size, 100*time.Millisecond)

		var insertedKeys atomic.Int64
		// random val != 0
		insertedKeys.Store(13)

		done := make(chan struct{})
		go func() {
			var key int64
			for {
				select {
				case <-done:
					return
				case <-time.After(time.Duration((rand.IntN(10))+1) * time.Millisecond):
					tc.Add(&key)
					key += 1
					insertedKeys.Store(key)
				}
			}
		}()

		b.ResetTimer()
		for range b.N {
			n := insertedKeys.Load()

			key := rand.Int64N(n)
			tc.Get(&key)
		}

		b.StopTimer()
		close(done)
	})
}
