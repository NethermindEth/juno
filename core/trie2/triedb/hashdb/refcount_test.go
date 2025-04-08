package hashdb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRefCountCache_BasicOperations(t *testing.T) {
	cache := NewRefCountCache()

	assert.Equal(t, 0, cache.Len())
	assert.Equal(t, uint64(0), cache.Hits())
	assert.Equal(t, float64(0), cache.HitRate())

	key1 := []byte("key1")
	node1 := cachedNode{
		blob:     []byte("node1"),
		parents:  0,
		external: make(map[string]struct{}),
	}

	success := cache.Set(key1, node1)
	assert.True(t, success)

	retrievedNode, found := cache.Get(key1)
	assert.True(t, found)
	assert.Equal(t, node1.blob, retrievedNode.blob)
	assert.Equal(t, uint64(1), cache.Hits())

	_, found = cache.Get([]byte("nonexistent"))
	assert.False(t, found)
	assert.Equal(t, uint64(1), cache.Hits())
	assert.Equal(t, uint64(1), cache.misses)

	assert.Equal(t, float64(0.5), cache.HitRate())

	success = cache.Remove(key1)
	assert.True(t, success)
	assert.Equal(t, 0, cache.Len())

	success = cache.Remove([]byte("nonexistent"))
	assert.False(t, success)
}

func TestRefCountCache_FlushList(t *testing.T) {
	cache := NewRefCountCache()

	key1 := []byte("key1")
	key2 := []byte("key2")
	key3 := []byte("key3")

	node1 := cachedNode{blob: []byte("node1"), external: make(map[string]struct{})}
	node2 := cachedNode{blob: []byte("node2"), external: make(map[string]struct{})}
	node3 := cachedNode{blob: []byte("node3"), external: make(map[string]struct{})}

	cache.Set(key1, node1)
	cache.Set(key2, node2)
	cache.Set(key3, node3)

	assert.Equal(t, string(key1), cache.oldest)
	assert.Equal(t, string(key3), cache.newest)

	oldestKey, oldestValue, ok := cache.GetOldest()
	assert.True(t, ok)
	assert.Equal(t, key1, oldestKey)
	assert.Equal(t, node1.blob, oldestValue.blob)

	success := cache.RemoveOldest()
	assert.True(t, success)

	oldestKey, oldestValue, ok = cache.GetOldest()
	assert.True(t, ok)
	assert.Equal(t, key2, oldestKey)
	assert.Equal(t, node2.blob, oldestValue.blob)

	success = cache.Remove(key2)
	assert.True(t, success)

	assert.Equal(t, string(key3), cache.oldest)
	assert.Equal(t, string(key3), cache.newest)
}

func TestRefCountCache_ReferenceAndDereference(t *testing.T) {
	cache := NewRefCountCache()

	parentKey := []byte("parent")
	child1Key := []byte("child1")
	child2Key := []byte("child2")

	parentNode := cachedNode{blob: []byte("parent data"), external: make(map[string]struct{})}
	child1Node := cachedNode{blob: []byte("child1 data"), external: make(map[string]struct{})}
	child2Node := cachedNode{blob: []byte("child2 data"), external: make(map[string]struct{})}

	cache.Set(parentKey, parentNode)
	cache.Set(child1Key, child1Node)
	cache.Set(child2Key, child2Node)

	cache.Reference(child1Key, parentKey)
	cache.Reference(child2Key, parentKey)

	parent, found := cache.Get(parentKey)
	require.True(t, found)
	assert.Contains(t, parent.external, string(child1Key))
	assert.Contains(t, parent.external, string(child2Key))

	child1, found := cache.Get(child1Key)
	require.True(t, found)
	assert.Equal(t, uint32(1), child1.parents)

	child2, found := cache.Get(child2Key)
	require.True(t, found)
	assert.Equal(t, uint32(1), child2.parents)

	rootKey := []byte("root")
	rootNode := cachedNode{blob: []byte("root data"), external: make(map[string]struct{})}

	cache.Set(rootKey, rootNode)
	cache.Reference(rootKey, nil)

	root, found := cache.Get(rootKey)
	require.True(t, found)
	assert.Equal(t, uint32(1), root.parents)

	cache.Dereference(parentKey)

	_, found = cache.Get(parentKey)
	assert.False(t, found)

	_, found = cache.Get(child1Key)
	assert.False(t, found)

	_, found = cache.Get(child2Key)
	assert.False(t, found)

	_, found = cache.Get(rootKey)
	assert.True(t, found)

	cache.Dereference(rootKey)

	_, found = cache.Get(rootKey)
	assert.False(t, found)
}

func TestRefCountCache_ComplexReferences(t *testing.T) {
	cache := NewRefCountCache()

	keysA := []byte("nodeA")
	keysB := []byte("nodeB")
	keysC := []byte("nodeC")
	keysD := []byte("nodeD")
	keysE := []byte("nodeE")

	nodeA := cachedNode{blob: []byte("nodeA data"), external: make(map[string]struct{})}
	nodeB := cachedNode{blob: []byte("nodeB data"), external: make(map[string]struct{})}
	nodeC := cachedNode{blob: []byte("nodeC data"), external: make(map[string]struct{})}
	nodeD := cachedNode{blob: []byte("nodeD data"), external: make(map[string]struct{})}
	nodeE := cachedNode{blob: []byte("nodeE data"), external: make(map[string]struct{})}

	cache.Set(keysA, nodeA)
	cache.Set(keysB, nodeB)
	cache.Set(keysC, nodeC)
	cache.Set(keysD, nodeD)
	cache.Set(keysE, nodeE)

	cache.Reference(keysB, keysA)
	cache.Reference(keysC, keysB)
	cache.Reference(keysD, keysA)
	cache.Reference(keysB, keysE)

	nodeAVal, _ := cache.Get(keysA)
	nodeBVal, _ := cache.Get(keysB)
	nodeCVal, _ := cache.Get(keysC)
	nodeDVal, _ := cache.Get(keysD)
	nodeEVal, _ := cache.Get(keysE)

	assert.Equal(t, uint32(0), nodeAVal.parents)
	assert.Equal(t, uint32(2), nodeBVal.parents)
	assert.Equal(t, uint32(1), nodeCVal.parents)
	assert.Equal(t, uint32(1), nodeDVal.parents)
	assert.Equal(t, uint32(0), nodeEVal.parents)

	cache.Dereference(keysE)

	nodeBVal, found := cache.Get(keysB)
	require.True(t, found)
	assert.Equal(t, uint32(1), nodeBVal.parents)

	cache.Dereference(keysA)

	_, found = cache.Get(keysA)
	assert.False(t, found)

	_, found = cache.Get(keysB)
	assert.False(t, found)

	_, found = cache.Get(keysC)
	assert.False(t, found)

	_, found = cache.Get(keysD)
	assert.False(t, found)

	_, found = cache.Get(keysE)
	assert.False(t, found)
}

func TestRefCountCache_CyclicReferences(t *testing.T) {
	cache := NewRefCountCache()

	keysA := []byte("nodeA")
	keysB := []byte("nodeB")
	keysC := []byte("nodeC")

	nodeA := cachedNode{blob: []byte("nodeA data"), external: make(map[string]struct{})}
	nodeB := cachedNode{blob: []byte("nodeB data"), external: make(map[string]struct{})}
	nodeC := cachedNode{blob: []byte("nodeC data"), external: make(map[string]struct{})}

	cache.Set(keysA, nodeA)
	cache.Set(keysB, nodeB)
	cache.Set(keysC, nodeC)

	cache.Reference(keysB, keysA)
	cache.Reference(keysC, keysB)
	cache.Reference(keysA, keysC)

	nodeAVal, _ := cache.Get(keysA)
	nodeBVal, _ := cache.Get(keysB)
	nodeCVal, _ := cache.Get(keysC)

	assert.Equal(t, uint32(1), nodeAVal.parents)
	assert.Equal(t, uint32(1), nodeBVal.parents)
	assert.Equal(t, uint32(1), nodeCVal.parents)

	cache.Reference(keysA, nil)
	nodeAVal, _ = cache.Get(keysA)
	assert.Equal(t, uint32(2), nodeAVal.parents)

	cache.Dereference(keysA)
	nodeAVal, _ = cache.Get(keysA)
	assert.Equal(t, uint32(1), nodeAVal.parents)

	cache.Dereference(keysC)

	_, foundA := cache.Get(keysA)
	_, foundB := cache.Get(keysB)
	_, foundC := cache.Get(keysC)

	assert.False(t, foundA)
	assert.False(t, foundB)
	assert.False(t, foundC)
}

func TestRefCountCache_EdgeCases(t *testing.T) {
	cache := NewRefCountCache()

	_, _, ok := cache.GetOldest()
	assert.False(t, ok)

	success := cache.RemoveOldest()
	assert.False(t, success)

	cache.Dereference([]byte("nonexistent"))
	assert.Equal(t, 0, cache.Len())

	cache.Reference([]byte("nonexistent"), []byte("parent"))
	assert.Equal(t, 0, cache.Len())

	key := []byte("key")
	node := cachedNode{blob: []byte("data"), external: make(map[string]struct{})}
	cache.Set(key, node)
	cache.Reference(key, []byte("nonexistent"))

	nodeVal, found := cache.Get(key)
	require.True(t, found)
	assert.Equal(t, uint32(0), nodeVal.parents)

	success = cache.Set(key, node)
	assert.False(t, success)
	assert.Equal(t, 1, cache.Len())

	success = cache.Remove(key)
	assert.True(t, success)
	assert.Equal(t, 0, cache.Len())
	assert.Equal(t, "", cache.oldest)
	assert.Equal(t, "", cache.newest)
}

func TestRefCountCache_MultipleReferences(t *testing.T) {
	cache := NewRefCountCache()

	keysA := []byte("nodeA")
	keysB := []byte("nodeB")
	keysC := []byte("nodeC")

	nodeA := cachedNode{blob: []byte("nodeA data"), external: make(map[string]struct{})}
	nodeB := cachedNode{blob: []byte("nodeB data"), external: make(map[string]struct{})}
	nodeC := cachedNode{blob: []byte("nodeC data"), external: make(map[string]struct{})}

	cache.Set(keysA, nodeA)
	cache.Set(keysB, nodeB)
	cache.Set(keysC, nodeC)

	cache.Reference(keysC, keysA)
	cache.Reference(keysC, keysB)

	nodeCVal, _ := cache.Get(keysC)
	assert.Equal(t, uint32(2), nodeCVal.parents)

	cache.Dereference(keysA)

	_, foundA := cache.Get(keysA)
	assert.False(t, foundA)

	nodeCVal, foundC := cache.Get(keysC)
	assert.True(t, foundC)
	assert.Equal(t, uint32(1), nodeCVal.parents)

	cache.Dereference(keysB)

	_, foundB := cache.Get(keysB)
	_, foundC = cache.Get(keysC)
	assert.False(t, foundB)
	assert.False(t, foundC)
}

func TestRefCountCache_ReferenceToSameNodeMultipleTimes(t *testing.T) {
	cache := NewRefCountCache()

	parent := []byte("parent")
	child := []byte("child")

	parentNode := cachedNode{blob: []byte("parent data"), external: make(map[string]struct{})}
	childNode := cachedNode{blob: []byte("child data"), external: make(map[string]struct{})}

	cache.Set(parent, parentNode)
	cache.Set(child, childNode)

	cache.Reference(child, parent)
	cache.Reference(child, parent)

	childVal, _ := cache.Get(child)
	assert.Equal(t, uint32(1), childVal.parents)

	parentVal, _ := cache.Get(parent)
	assert.Equal(t, 1, len(parentVal.external))
}

func TestRefCountCache_DereferenceWithMultipleChildren(t *testing.T) {
	cache := NewRefCountCache()

	keysA := []byte("nodeA")
	keysB := []byte("nodeB")
	keysC := []byte("nodeC")
	keysD := []byte("nodeD")
	keysE := []byte("nodeE")

	nodeA := cachedNode{blob: []byte("nodeA data"), external: make(map[string]struct{})}
	nodeB := cachedNode{blob: []byte("nodeB data"), external: make(map[string]struct{})}
	nodeC := cachedNode{blob: []byte("nodeC data"), external: make(map[string]struct{})}
	nodeD := cachedNode{blob: []byte("nodeD data"), external: make(map[string]struct{})}
	nodeE := cachedNode{blob: []byte("nodeE data"), external: make(map[string]struct{})}

	cache.Set(keysA, nodeA)
	cache.Set(keysB, nodeB)
	cache.Set(keysC, nodeC)
	cache.Set(keysD, nodeD)
	cache.Set(keysE, nodeE)

	cache.Reference(keysB, keysA)
	cache.Reference(keysC, keysB)
	cache.Reference(keysD, keysB)
	cache.Reference(keysE, keysA)

	cache.Reference(keysB, nil)

	cache.Dereference(keysA)

	_, foundA := cache.Get(keysA)
	_, foundE := cache.Get(keysE)
	assert.False(t, foundA)
	assert.False(t, foundE)

	nodeBVal, foundB := cache.Get(keysB)
	_, foundC := cache.Get(keysC)
	_, foundD := cache.Get(keysD)
	assert.True(t, foundB)
	assert.True(t, foundC)
	assert.True(t, foundD)
	assert.Equal(t, uint32(1), nodeBVal.parents)

	cache.Dereference(keysB)

	_, foundB = cache.Get(keysB)
	_, foundC = cache.Get(keysC)
	_, foundD = cache.Get(keysD)
	assert.False(t, foundB)
	assert.False(t, foundC)
	assert.False(t, foundD)
}
