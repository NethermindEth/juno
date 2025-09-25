package commontrie_test

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state/commontrie"
	oldtrie "github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/core/trie2"
	newtrie "github.com/NethermindEth/juno/core/trie2"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrieAdapter(t *testing.T) {
	trie, err := trie2.NewEmptyPedersen()
	require.NoError(t, err)
	adapter := commontrie.NewTrieAdapter(trie)

	t.Run("Update", func(t *testing.T) {
		err := adapter.Update(&felt.Zero, &felt.Zero)
		require.NoError(t, err)
	})

	t.Run("Get", func(t *testing.T) {
		err := adapter.Update(&felt.Zero, &felt.Zero)
		require.NoError(t, err)

		gotValue, err := adapter.Get(&felt.Zero)
		require.NoError(t, err)
		assert.Equal(t, felt.Zero, gotValue)
	})

	t.Run("Hash", func(t *testing.T) {
		hash, err := adapter.Hash()
		require.NoError(t, err)
		assert.Equal(t, felt.Zero, hash)
	})

	t.Run("HashFn", func(t *testing.T) {
		hashFn := adapter.HashFn()
		assert.NotNil(t, hashFn)
	})
}

type kv struct{ k, v felt.Felt }

// Deterministic random felt from a math/rand RNG
func randFeltFrom(r *rand.Rand) felt.Felt {
	var b [32]byte
	_, _ = r.Read(b[:])
	var f felt.Felt
	f.SetBytes(b[:])
	return f
}

// Make a diff set:
// - prefixBytes: 0 => scattered; bigger => more clustered (same MSB bytes)
// - deleteEvery: if >0, every Nth item becomes a delete (v=0)
func makeDiffs(n int, seed int64, prefixBytes int, deleteEvery int) []kv {
	if prefixBytes < 0 || prefixBytes > 32 {
		panic("prefixBytes must be in [0,32]")
	}
	r := rand.New(rand.NewSource(seed))

	var prefix [32]byte
	_, _ = r.Read(prefix[:])

	out := make([]kv, n)
	for i := 0; i < n; i++ {
		var keyB [32]byte
		_, _ = r.Read(keyB[:])
		copy(keyB[:prefixBytes], prefix[:prefixBytes])

		var k felt.Felt
		k.SetBytes(keyB[:])

		var v felt.Felt
		if deleteEvery > 0 && i%deleteEvery == 0 {
			v = felt.FromUint64(0)
		} else {
			var tmp [32]byte
			binary.BigEndian.PutUint64(tmp[24:], uint64(i+1))
			v.SetBytes(tmp[:])
		}

		out[i] = kv{k, v}
	}

	// Mild shuffle
	r.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

func applyUpdates(t commontrie.Trie, diffs []kv) error {
	for i := range diffs {
		if err := t.Update(&diffs[i].k, &diffs[i].v); err != nil {
			return err
		}
	}
	return nil
}

// factories
func newDeprecatedAdapter(tb testing.TB) commontrie.Trie {
	tb.Helper()
	mem := memory.New()
	storage := oldtrie.NewStorage(mem.NewIndexedBatch(), nil)
	tr, err := oldtrie.NewTriePedersen(storage, 251)
	if err != nil {
		tb.Fatalf("old trie init failed: %v", err)
	}
	return commontrie.NewDeprecatedTrieAdapter(tr)
}

func newNewAdapter(tb testing.TB) commontrie.Trie {
	tb.Helper()
	tr, err := newtrie.NewEmptyPedersen()
	if err != nil {
		tb.Fatalf("new trie init failed: %v", err)
	}
	return commontrie.NewTrieAdapter(tr)
}

type impl struct {
	name    string
	factory func(testing.TB) commontrie.Trie
}

var implementations = []impl{
	{name: "Deprecated", factory: newDeprecatedAdapter},
	{name: "New", factory: newNewAdapter},
}

func Benchmark_Block_ApplyAndHash_FreshKeys(b *testing.B) {
	type cfg struct {
		blocks      int
		blockSize   int
		prefixBytes int
		deleteEvery int
	}
	configs := []cfg{
		{blocks: 100, blockSize: 100, prefixBytes: 0, deleteEvery: 20},
		{blocks: 50, blockSize: 1000, prefixBytes: 0, deleteEvery: 20},
		{blocks: 50, blockSize: 1000, prefixBytes: 4, deleteEvery: 20},
		{blocks: 20, blockSize: 5000, prefixBytes: 8, deleteEvery: 20},
	}

	for _, c := range configs {
		title := fmt.Sprintf("B=%d,Nb=%d,prefix=%d,del=%d",
			c.blocks, c.blockSize, c.prefixBytes, c.deleteEvery)
		b.Run(title, func(b *testing.B) {
			for _, impl := range implementations {
				b.Run(impl.name, func(b *testing.B) {
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						tr := impl.factory(b)
						for blk := 0; blk < c.blocks; blk++ {
							// new keys each block
							diffs := makeDiffs(c.blockSize, int64(0xB10C<<8|blk), c.prefixBytes, c.deleteEvery)
							if err := applyUpdates(tr, diffs); err != nil {
								b.Fatalf("updates: %v", err)
							}
							if _, err := tr.Hash(); err != nil { // commit for the block
								b.Fatalf("hash: %v", err)
							}
						}
					}
				})
			}
		})
	}
}

func Benchmark_Block_ApplyAndHash_Overlap(b *testing.B) {
	type cfg struct {
		blocks        int
		blockSize     int
		prefixBytes   int
		deleteEvery   int
		overwriteFrac float64
	}
	configs := []cfg{
		{blocks: 50, blockSize: 1000, prefixBytes: 4, deleteEvery: 20, overwriteFrac: 0.50},
		{blocks: 50, blockSize: 1000, prefixBytes: 8, deleteEvery: 20, overwriteFrac: 0.80},
	}

	for _, c := range configs {
		title := fmt.Sprintf("B=%d,Nb=%d,prefix=%d,del=%d,overw=%.0f%%",
			c.blocks, c.blockSize, c.prefixBytes, c.deleteEvery, 100*c.overwriteFrac)
		b.Run(title, func(b *testing.B) {
			for _, impl := range implementations {
				b.Run(impl.name, func(b *testing.B) {
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						tr := impl.factory(b)

						// start with an initial block of keys
						cur := makeDiffs(c.blockSize, time.Now().UnixNano(), c.prefixBytes, c.deleteEvery)
						if err := applyUpdates(tr, cur); err != nil {
							b.Fatalf("prime: %v", err)
						}
						if _, err := tr.Hash(); err != nil {
							b.Fatalf("prime hash: %v", err)
						}

						r := rand.New(rand.NewSource(1337))
						for blk := 1; blk < c.blocks; blk++ {
							// choose a subset to overwrite
							overwCount := int(float64(len(cur)) * c.overwriteFrac)
							next := make([]kv, 0, c.blockSize)

							perm := r.Perm(len(cur))
							for i := 0; i < overwCount; i++ {
								idx := perm[i]
								// same key, new value (or zero if we hit deleteEvery)
								nv := randFeltFrom(r)
								next = append(next, kv{cur[idx].k, nv})
							}
							// fill the rest with fresh keys
							fresh := makeDiffs(c.blockSize-overwCount, int64(0xFEED<<8|blk), c.prefixBytes, c.deleteEvery)
							next = append(next, fresh...)

							if err := applyUpdates(tr, next); err != nil {
								b.Fatalf("updates: %v", err)
							}
							if _, err := tr.Hash(); err != nil { // commit per block
								b.Fatalf("hash: %v", err)
							}
							cur = next
						}
					}
				})
			}
		})
	}
}
