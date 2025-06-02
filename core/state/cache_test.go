package state

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/assert"
)

func TestStateCache(t *testing.T) {
	t.Run("new state cache is empty", func(t *testing.T) {
		cache := newStateCache()
		assert.Empty(t, cache.diffs)
		assert.Empty(t, cache.links)
	})

	t.Run("add layer and retrieve data", func(t *testing.T) {
		cache := newStateCache()
		root := new(felt.Felt).SetUint64(1)
		parent := new(felt.Felt).SetUint64(0)
		addr := new(felt.Felt).SetUint64(100)
		nonce := new(felt.Felt).SetUint64(5)
		classHash := new(felt.Felt).SetUint64(200)
		storageKey := new(felt.Felt).SetUint64(300)
		storageValue := new(felt.Felt).SetUint64(400)

		diff := &diffCache{
			storageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
				*addr: {
					*storageKey: storageValue,
				},
			},
			nonces: map[felt.Felt]*felt.Felt{
				*addr: nonce,
			},
			deployedContracts: map[felt.Felt]*felt.Felt{
				*addr: classHash,
			},
		}

		cache.AddLayer(*root, *parent, diff)

		// Test retrieving data
		assert.Equal(t, nonce, cache.getNonce(root, addr))
		assert.Equal(t, storageValue, cache.getStorageDiff(root, addr)[*storageKey])
		assert.Equal(t, classHash, cache.getDeployedContract(root, addr))
	})

	t.Run("layer eviction", func(t *testing.T) {
		cache := newStateCache()
		roots := make([]felt.Felt, DefaultMaxLayers+2)
		parent := new(felt.Felt).SetUint64(0)

		// Add more layers than DefaultMaxLayers
		for i := range roots {
			roots[i] = *new(felt.Felt).SetUint64(uint64(i + 1))
			diff := &diffCache{
				nonces: map[felt.Felt]*felt.Felt{
					*new(felt.Felt).SetUint64(uint64(i + 100)): new(felt.Felt).SetUint64(uint64(i + 1)),
				},
			}
			cache.AddLayer(roots[i], *parent, diff)
			parent = &roots[i]
		}

		// Verify that oldest layers are evicted
		assert.Equal(t, DefaultMaxLayers, len(cache.links))
		assert.Equal(t, DefaultMaxLayers, len(cache.diffs))
		assert.Equal(t, roots[1], cache.oldestRoot) // First layer should be evicted
	})

	t.Run("parent chain traversal", func(t *testing.T) {
		cache := newStateCache()
		root1 := new(felt.Felt).SetUint64(1)
		root2 := new(felt.Felt).SetUint64(2)
		root3 := new(felt.Felt).SetUint64(3)
		addr := new(felt.Felt).SetUint64(100)
		nonce1 := new(felt.Felt).SetUint64(5)
		nonce2 := new(felt.Felt).SetUint64(6)

		// Create a chain of layers with different nonces
		diff1 := &diffCache{
			nonces: map[felt.Felt]*felt.Felt{
				*addr: nonce1,
			},
		}
		diff2 := &diffCache{
			nonces: map[felt.Felt]*felt.Felt{
				*addr: nonce2,
			},
		}
		diff3 := &diffCache{}

		cache.AddLayer(*root1, felt.Zero, diff1)
		cache.AddLayer(*root2, *root1, diff2)
		cache.AddLayer(*root3, *root2, diff3)

		// Test that we can traverse up the chain to find values
		assert.Equal(t, nonce2, cache.getNonce(root3, addr)) // Should find in root2
		assert.Equal(t, nonce1, cache.getNonce(root2, addr)) // Should find in root1
		assert.Equal(t, nonce1, cache.getNonce(root1, addr)) // Should find in root1
	})

	t.Run("non-existent data returns nil", func(t *testing.T) {
		cache := newStateCache()
		root := new(felt.Felt).SetUint64(1)
		addr := new(felt.Felt).SetUint64(100)

		assert.Nil(t, cache.getNonce(root, addr))
		assert.Nil(t, cache.getStorageDiff(root, addr))
		assert.Nil(t, cache.getDeployedContract(root, addr))
	})
}
