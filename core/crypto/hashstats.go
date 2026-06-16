package crypto

// HARDWIRED hash-redundancy instrumentation. MEASUREMENT BRANCH ONLY — never merge.
//
// For every Poseidon permutation and every Pedersen hash it records the input's
// 64-bit fingerprint into simulated LRU caches of several sizes (set-associative,
// allocation-free, striped locks) and exposes hit counters as Prometheus metrics.
// Everything is bounded in memory (independent of the number of hashes), so it
// survives a full sync from genesis. Scrape /metrics for the time series:
//
//	primitives_hash_requests_total{family}
//	primitives_hash_cache_hits_total{family,size}
//
// hit-rate(size) = rate(cache_hits{size}) / rate(requests); correlate with chain height.

import (
	"sync"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	assocWays   = 8
	statsStripe = 256

	// FNV-1a 64-bit constants, for fingerprinting hash inputs.
	fnvOffset64 = 1469598103934665603
	fnvPrime64  = 1099511628211
)

// assocCache is a fixed-size set-associative LRU approximation. No per-access
// allocation; recency tracked by a per-set monotonic tick, evicting the minimum.
type assocCache struct {
	sets  uint64
	keys  []uint64 // sets*assocWays, 0 means empty slot
	ticks []uint64 // sets*assocWays
	clock []uint64 // per-set tick
	locks [statsStripe]sync.Mutex
}

func newAssocCache(capacity int) *assocCache {
	sets := uint64(capacity / assocWays)
	if sets < 1 {
		sets = 1
	}
	return &assocCache{
		sets:  sets,
		keys:  make([]uint64, sets*assocWays),
		ticks: make([]uint64, sets*assocWays),
		clock: make([]uint64, sets),
	}
}

func (c *assocCache) access(key uint64) bool {
	if key == 0 {
		key = 1 // 0 is the empty-slot sentinel
	}
	set := key % c.sets
	lk := &c.locks[set%statsStripe]
	lk.Lock()
	defer lk.Unlock()

	c.clock[set]++
	now := c.clock[set]
	base := set * assocWays
	minIdx := base
	minTick := c.ticks[base]
	for i := base; i < base+assocWays; i++ {
		switch {
		case c.keys[i] == key:
			c.ticks[i] = now
			return true
		case c.keys[i] == 0:
			c.keys[i], c.ticks[i] = key, now
			return false
		case c.ticks[i] < minTick:
			minTick, minIdx = c.ticks[i], i
		}
	}
	c.keys[minIdx], c.ticks[minIdx] = key, now
	return false
}

var (
	statsSizes  = []int{1 << 10, 1 << 16, 1 << 20}
	statsLabels = []string{"1k", "64k", "1m"}

	// promauto registers on the default registry at creation (no init needed).
	hashRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "primitives",
		Subsystem: "hash",
		Name:      "requests_total",
		Help:      "Hash permutations recorded, by family.",
	}, []string{"family"})

	hashCacheHits = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "primitives",
		Subsystem: "hash",
		Name:      "cache_hits_total",
		Help:      "Simulated LRU cache hits, by family and cache size.",
	}, []string{"family", "size"})

	// Poseidon's empty-data input ({0,1,0}) is a trivial constant we already
	// short-circuit; count it separately and exclude it from the cache stats so
	// hit-rate reflects the non-trivial redundancy.
	hashEmpty = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "primitives",
		Subsystem: "hash",
		Name:      "poseidon_empty_total",
		Help:      "Poseidon empty-data permutation requests (excluded from cache stats).",
	})
)

type familyStats struct {
	caches []*assocCache
	total  prometheus.Counter
	hits   []prometheus.Counter
}

func newFamilyStats(name string) *familyStats {
	f := &familyStats{total: hashRequests.WithLabelValues(name)}
	for i, size := range statsSizes {
		f.caches = append(f.caches, newAssocCache(size))
		f.hits = append(f.hits, hashCacheHits.WithLabelValues(name, statsLabels[i]))
	}
	return f
}

func (f *familyStats) record(key uint64) {
	f.total.Inc()
	for i, c := range f.caches {
		if c.access(key) {
			f.hits[i].Inc()
		}
	}
}

var (
	poseidonStats = newFamilyStats("poseidon")
	pedersenStats = newFamilyStats("pedersen")
)

func fnvMix(h, w uint64) uint64 { return (h ^ w) * fnvPrime64 }

func recordPoseidon(state *[3]felt.Felt) {
	if *state == emptyDataInput { // trivial constant, short-circuited; track separately
		hashEmpty.Inc()
		return
	}
	h := uint64(fnvOffset64)
	for i := range state {
		h = fnvMix(h, state[i][0])
		h = fnvMix(h, state[i][1])
		h = fnvMix(h, state[i][2])
		h = fnvMix(h, state[i][3])
	}
	poseidonStats.record(h)
}

func recordPedersen(a, b *fp.Element) {
	h := uint64(fnvOffset64)
	h = fnvMix(h, a[0])
	h = fnvMix(h, a[1])
	h = fnvMix(h, a[2])
	h = fnvMix(h, a[3])
	h = fnvMix(h, b[0])
	h = fnvMix(h, b[1])
	h = fnvMix(h, b[2])
	h = fnvMix(h, b[3])
	pedersenStats.record(h)
}
