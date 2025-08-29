package commontrie

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/core/trie2"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrieAdapter(t *testing.T) {
	trie, err := trie2.NewEmptyPedersen()
	require.NoError(t, err)
	adapter := NewTrieAdapter(trie)

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

func BenchmarkTrieAdapters(b *testing.B) {
	benchmarkData := make(map[felt.Felt]felt.Felt)
	for i := 0; i < 10000; i++ {
		key, err := new(felt.Felt).SetRandom()
		require.NoError(b, err)
		value, err := new(felt.Felt).SetRandom()
		require.NoError(b, err)
		benchmarkData[*key] = *value
	}

	b.Run("DeprecatedTrieAdapter", func(b *testing.B) {
		memDB := memory.New()
		txn := memDB.NewIndexedBatch()
		storage := trie.NewStorage(txn, db.ContractStorage.Key([]byte{0}))
		trie, err := trie.NewTriePedersen(storage, 251)
		if err != nil {
			b.Fatalf("Failed to create trie: %v", err)
		}
		adapter := NewDeprecatedTrieAdapter(trie)

		b.Run("Update", func(b *testing.B) {
			for b.Loop() {
				for key, value := range benchmarkData {
					if key.Uint64()%20 == 0 {
						value = felt.FromUint64(0)
					}
					if err := adapter.Update(&key, &value); err != nil {
						b.Fatalf("Update failed: %v", err)
					}
				}
			}
		})

		b.Run("Get", func(b *testing.B) {
			for b.Loop() {
				for key := range benchmarkData {
					if _, err := adapter.Get(&key); err != nil {
						b.Fatalf("Get failed: %v", err)
					}
				}
			}
		})

		b.Run("Hash", func(b *testing.B) {
			for b.Loop() {
				if _, err := adapter.Hash(); err != nil {
					b.Fatalf("Hash failed: %v", err)
				}
			}
		})
	})

	b.Run("TrieAdapter", func(b *testing.B) {
		trie, err := trie2.NewEmptyPedersen()
		if err != nil {
			b.Fatalf("Failed to create trie: %v", err)
		}
		adapter := NewTrieAdapter(trie)

		b.Run("Update", func(b *testing.B) {
			for b.Loop() {
				for key, value := range benchmarkData {
					if key.Uint64()%20 == 0 {
						value = felt.FromUint64(0)
					}
					if err := adapter.Update(&key, &value); err != nil {
						b.Fatalf("Update failed: %v", err)
					}
				}
			}
		})

		b.Run("Get", func(b *testing.B) {
			for b.Loop() {
				for key := range benchmarkData {
					if _, err := adapter.Get(&key); err != nil {
						b.Fatalf("Get failed: %v", err)
					}
				}
			}
		})

		b.Run("Hash", func(b *testing.B) {
			for b.Loop() {
				if _, err := adapter.Hash(); err != nil {
					b.Fatalf("Hash failed: %v", err)
				}
			}
		})
	})
}
