package lru

import (
	"math/rand/v2"
	"testing"

	gethlru "github.com/ethereum/go-ethereum/common/lru"
	hashilru "github.com/hashicorp/golang-lru/v2"
	hashisimple "github.com/hashicorp/golang-lru/v2/simplelru"
)

// Paired benchmarks: ethereum/common/lru (old) vs hashicorp/golang-lru/v2 (new).
// Key shapes match real callsites:
//   - int                   baseline
//   - bloomKey {u64, u64}   blockchain.EventFiltersCacheKey
//   - feltKey  [4]uint64    rpccore.TraceCacheKey (felt.Felt underlies as [4]uint64)
//
// Sizes mirror production:
//   - 16   AggregatedBloomFilterCacheSize
//   - 128  TraceCacheSize
//   - 8192 stress
//
// Run with: go test -bench=. -benchmem -count=10 ./utils/lru/

type bloomKey struct {
	from uint64
	to   uint64
}

type feltKey [4]uint64

// generic cache interface so benchmarks can be written once.
type lruCache[K comparable] interface {
	Add(key K, value int) (evicted bool)
	Get(key K) (value int, ok bool)
}

// adapters

type gethCache[K comparable] struct{ c *gethlru.Cache[K, int] }

func (g gethCache[K]) Add(k K, v int) bool { return g.c.Add(k, v) }
func (g gethCache[K]) Get(k K) (int, bool) { return g.c.Get(k) }

type hashiCache[K comparable] struct{ c *hashilru.Cache[K, int] }

func (h hashiCache[K]) Add(k K, v int) bool { evicted := h.c.Add(k, v); return evicted }
func (h hashiCache[K]) Get(k K) (int, bool) { return h.c.Get(k) }

func newGeth[K comparable](size int) lruCache[K] {
	return gethCache[K]{c: gethlru.NewCache[K, int](size)}
}

func newHashi[K comparable](size int) lruCache[K] {
	c, err := hashilru.New[K, int](size)
	if err != nil {
		panic(err)
	}
	return hashiCache[K]{c: c}
}

// key generators

func intKey(i int) int            { return i }
func bloomFromInt(i int) bloomKey { return bloomKey{from: uint64(i), to: uint64(i) + 1024} }
func feltFromInt(i int) feltKey {
	return feltKey{uint64(i), uint64(i) >> 1, uint64(i) << 1, ^uint64(i)}
}

// per-impl, per-keytype matrix runner

type implCase[K comparable] struct {
	name string
	make func(int) lruCache[K]
}

func impls[K comparable]() []implCase[K] {
	return []implCase[K]{
		{name: "geth", make: newGeth[K]},
		{name: "hashi", make: newHashi[K]},
	}
}

// --- Add-only (write-heavy with evictions) ---

func benchAdd[K comparable](b *testing.B, size int, gen func(int) K) {
	for _, imp := range impls[K]() {
		b.Run(imp.name, func(b *testing.B) {
			c := imp.make(size)
			b.ReportAllocs()
			b.ResetTimer()
			for i := range b.N {
				c.Add(gen(i), i)
			}
		})
	}
}

func BenchmarkAdd_Int_128(b *testing.B)  { benchAdd[int](b, 128, intKey) }
func BenchmarkAdd_Int_8192(b *testing.B) { benchAdd[int](b, 8192, intKey) }
func BenchmarkAdd_Bloom_16(b *testing.B) { benchAdd[bloomKey](b, 16, bloomFromInt) }
func BenchmarkAdd_Felt_128(b *testing.B) { benchAdd[feltKey](b, 128, feltFromInt) }

// --- Get hit (read-heavy, all keys present) ---

func benchGetHit[K comparable](b *testing.B, size int, gen func(int) K) {
	for _, imp := range impls[K]() {
		b.Run(imp.name, func(b *testing.B) {
			c := imp.make(size)
			for i := range size {
				c.Add(gen(i), i)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := range b.N {
				_, _ = c.Get(gen(i % size))
			}
		})
	}
}

func BenchmarkGetHit_Int_128(b *testing.B)  { benchGetHit[int](b, 128, intKey) }
func BenchmarkGetHit_Int_8192(b *testing.B) { benchGetHit[int](b, 8192, intKey) }
func BenchmarkGetHit_Bloom_16(b *testing.B) { benchGetHit[bloomKey](b, 16, bloomFromInt) }
func BenchmarkGetHit_Felt_128(b *testing.B) { benchGetHit[feltKey](b, 128, feltFromInt) }

// --- Get miss (all lookups fail) ---

func benchGetMiss[K comparable](b *testing.B, size int, gen func(int) K) {
	for _, imp := range impls[K]() {
		b.Run(imp.name, func(b *testing.B) {
			c := imp.make(size)
			for i := range size {
				c.Add(gen(i), i)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := range b.N {
				_, _ = c.Get(gen(size + i))
			}
		})
	}
}

func BenchmarkGetMiss_Int_128(b *testing.B)  { benchGetMiss[int](b, 128, intKey) }
func BenchmarkGetMiss_Bloom_16(b *testing.B) { benchGetMiss[bloomKey](b, 16, bloomFromInt) }
func BenchmarkGetMiss_Felt_128(b *testing.B) { benchGetMiss[feltKey](b, 128, feltFromInt) }

// --- Mixed 80/20 read/write with random keys over 2*size space (~50% hit rate) ---

//nolint:dupl // benchSimpleMixed mirrors this shape over a different cache interface.
func benchMixed[K comparable](b *testing.B, size int, gen func(int) K) {
	for _, imp := range impls[K]() {
		b.Run(imp.name, func(b *testing.B) {
			c := imp.make(size)
			for i := range size {
				c.Add(gen(i), i)
			}
			rng := rand.New(rand.NewPCG(1, 2))
			space := size * 2
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				k := rng.IntN(space)
				if rng.IntN(10) < 8 {
					_, _ = c.Get(gen(k))
				} else {
					c.Add(gen(k), k)
				}
			}
		})
	}
}

func BenchmarkMixed_Int_128(b *testing.B)  { benchMixed[int](b, 128, intKey) }
func BenchmarkMixed_Bloom_16(b *testing.B) { benchMixed[bloomKey](b, 16, bloomFromInt) }
func BenchmarkMixed_Felt_128(b *testing.B) { benchMixed[feltKey](b, 128, feltFromInt) }

// --- Parallel mixed: measures lock contention ---

func benchParallelMixed[K comparable](b *testing.B, size int, gen func(int) K) {
	for _, imp := range impls[K]() {
		b.Run(imp.name, func(b *testing.B) {
			c := imp.make(size)
			for i := range size {
				c.Add(gen(i), i)
			}
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
				space := size * 2
				for pb.Next() {
					k := rng.IntN(space)
					if rng.IntN(10) < 8 {
						_, _ = c.Get(gen(k))
					} else {
						c.Add(gen(k), k)
					}
				}
			})
		})
	}
}

func BenchmarkParallel_Int_128(b *testing.B)  { benchParallelMixed[int](b, 128, intKey) }
func BenchmarkParallel_Bloom_16(b *testing.B) { benchParallelMixed[bloomKey](b, 16, bloomFromInt) }
func BenchmarkParallel_Felt_128(b *testing.B) { benchParallelMixed[feltKey](b, 128, feltFromInt) }

// --- Non-thread-safe variants (basiclru vs simplelru) ---
// Mirrors sync/pending_polling.go preLatest cache: felt key, size=10.
// No locks → measures pure data-structure cost.

type simpleCache[K comparable] interface {
	Add(key K, value int) bool
	Get(key K) (int, bool)
}

type gethBasic[K comparable] struct{ c gethlru.BasicLRU[K, int] }

func (g *gethBasic[K]) Add(k K, v int) bool { return g.c.Add(k, v) }
func (g *gethBasic[K]) Get(k K) (int, bool) { return g.c.Get(k) }

type hashiSimple[K comparable] struct{ c *hashisimple.LRU[K, int] }

func (h *hashiSimple[K]) Add(k K, v int) bool { return h.c.Add(k, v) }
func (h *hashiSimple[K]) Get(k K) (int, bool) { return h.c.Get(k) }

func newGethBasic[K comparable](size int) simpleCache[K] {
	c := gethlru.NewBasicLRU[K, int](size)
	return &gethBasic[K]{c: c}
}

func newHashiSimple[K comparable](size int) simpleCache[K] {
	c, err := hashisimple.NewLRU[K, int](size, nil)
	if err != nil {
		panic(err)
	}
	return &hashiSimple[K]{c: c}
}

type simpleImplCase[K comparable] struct {
	name string
	make func(int) simpleCache[K]
}

func simpleImpls[K comparable]() []simpleImplCase[K] {
	return []simpleImplCase[K]{
		{name: "geth-basic", make: newGethBasic[K]},
		{name: "hashi-simple", make: newHashiSimple[K]},
	}
}

func benchSimpleAdd[K comparable](b *testing.B, size int, gen func(int) K) {
	for _, imp := range simpleImpls[K]() {
		b.Run(imp.name, func(b *testing.B) {
			c := imp.make(size)
			b.ReportAllocs()
			b.ResetTimer()
			for i := range b.N {
				c.Add(gen(i), i)
			}
		})
	}
}

func benchSimpleGetHit[K comparable](b *testing.B, size int, gen func(int) K) {
	for _, imp := range simpleImpls[K]() {
		b.Run(imp.name, func(b *testing.B) {
			c := imp.make(size)
			for i := range size {
				c.Add(gen(i), i)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := range b.N {
				_, _ = c.Get(gen(i % size))
			}
		})
	}
}

//nolint:dupl // benchMixed mirrors this shape over a different cache interface.
func benchSimpleMixed[K comparable](b *testing.B, size int, gen func(int) K) {
	for _, imp := range simpleImpls[K]() {
		b.Run(imp.name, func(b *testing.B) {
			c := imp.make(size)
			for i := range size {
				c.Add(gen(i), i)
			}
			rng := rand.New(rand.NewPCG(1, 2))
			space := size * 2
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				k := rng.IntN(space)
				if rng.IntN(10) < 8 {
					_, _ = c.Get(gen(k))
				} else {
					c.Add(gen(k), k)
				}
			}
		})
	}
}

func BenchmarkSimpleAdd_Felt_10(b *testing.B)    { benchSimpleAdd[feltKey](b, 10, feltFromInt) }
func BenchmarkSimpleGetHit_Felt_10(b *testing.B) { benchSimpleGetHit[feltKey](b, 10, feltFromInt) }
func BenchmarkSimpleMixed_Felt_10(b *testing.B)  { benchSimpleMixed[feltKey](b, 10, feltFromInt) }
