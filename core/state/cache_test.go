package state

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateCache(t *testing.T) {
	t.Run("new state cache is empty", func(t *testing.T) {
		cache := newStateCache()
		assert.Empty(t, cache.rootMap)
		assert.Equal(t, 0, len(cache.rootMap))
		assert.Nil(t, cache.oldestNode)
		assert.Nil(t, cache.newestNode)
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

		cache.PushLayer(root, parent, diff)

		// Test retrieving data
		assert.Equal(t, nonce, cache.getNonce(root, addr))
		assert.Equal(t, storageValue, cache.getStorageDiff(root, addr, storageKey))
		assert.Equal(t, classHash, cache.getReplacedClass(root, addr))
	})

	t.Run("layer eviction", func(t *testing.T) {
		cache := newStateCache()
		parent := new(felt.Felt).SetUint64(0)

		// Add exactly DefaultMaxLayers + 1 layers to trigger eviction
		for i := range DefaultMaxLayers + 1 {
			root := new(felt.Felt).SetUint64(uint64(i + 1))
			diff := &diffCache{
				nonces: map[felt.Felt]*felt.Felt{
					*new(felt.Felt).SetUint64(uint64(i + 100)): new(felt.Felt).SetUint64(uint64(i + 1)),
				},
			}
			cache.PushLayer(root, parent, diff)
			parent = root
		}

		// Verify that oldest layers are evicted
		assert.Equal(t, DefaultMaxLayers, len(cache.rootMap))

		// After eviction, the oldest root should be the second layer (root 2)
		expectedOldestRoot := new(felt.Felt).SetUint64(2)
		assert.Equal(t, *expectedOldestRoot, cache.oldestNode.root)

		// Verify that the first layer is evicted by checking that its data is not accessible
		firstLayerRoot := new(felt.Felt).SetUint64(1)
		firstLayerAddr := new(felt.Felt).SetUint64(100)
		assert.Nil(t, cache.getNonce(firstLayerRoot, firstLayerAddr))
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

		cache.PushLayer(root1, &felt.Zero, diff1)
		cache.PushLayer(root2, root1, diff2)
		cache.PushLayer(root3, root2, diff3)

		// Test that we can traverse up the chain to find values
		assert.Equal(t, nonce2, cache.getNonce(root3, addr)) // Should find in root2
		assert.Equal(t, nonce2, cache.getNonce(root2, addr)) // Should find in root2
		assert.Equal(t, nonce1, cache.getNonce(root1, addr)) // Should find in root1
	})

	t.Run("non-existent data returns nil", func(t *testing.T) {
		cache := newStateCache()
		root := new(felt.Felt).SetUint64(1)
		addr := new(felt.Felt).SetUint64(100)

		assert.Nil(t, cache.getNonce(root, addr))
		assert.Nil(t, cache.getStorageDiff(root, addr, new(felt.Felt).SetUint64(1)))
		assert.Nil(t, cache.getReplacedClass(root, addr))
	})

	t.Run("pop from empty cache", func(t *testing.T) {
		cache := newStateCache()
		root := new(felt.Felt).SetUint64(1)

		err := cache.PopLayer(root, &felt.Zero)
		require.NoError(t, err)
	})

	t.Run("push and pop multiple layers with no changes", func(t *testing.T) {
		cache := newStateCache()
		parent := new(felt.Felt).SetUint64(0)

		emptyDiff := &diffCache{
			storageDiffs:      make(map[felt.Felt]map[felt.Felt]*felt.Felt),
			nonces:            make(map[felt.Felt]*felt.Felt),
			deployedContracts: make(map[felt.Felt]*felt.Felt),
		}

		var roots []*felt.Felt
		for i := range 3 {
			roots = append(roots, new(felt.Felt).SetUint64(uint64(1)))
			if i == 0 {
				cache.PushLayer(roots[i], parent, emptyDiff)
			} else {
				cache.PushLayer(roots[i], roots[i-1], emptyDiff)
			}
			assert.Equal(t, 1, len(cache.rootMap))
			assert.Equal(t, cache.oldestNode.root, cache.newestNode.root)
		}

		var err error
		for i := len(roots) - 1; i >= 0; i-- {
			if i == 0 {
				err = cache.PopLayer(roots[i], parent)
				require.NoError(t, err)
				assert.Equal(t, 0, len(cache.rootMap))
			} else {
				err = cache.PopLayer(roots[i], roots[i-1])
				assert.Equal(t, 1, len(cache.rootMap))
			}
			require.NoError(t, err)
		}

		assert.Empty(t, cache.rootMap)
		assert.Nil(t, cache.oldestNode)
		assert.Nil(t, cache.newestNode)
	})
}
