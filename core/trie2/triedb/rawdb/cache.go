package rawdb

import (
	"math"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/VictoriaMetrics/fastcache"
)

const nodeCacheSize = ownerSize + trieutils.PathSize + 1

type trieType byte

const (
	contract trieType = iota
	class
)

// Stores committed trie nodes in memory
type cleanCache struct {
	cache *fastcache.Cache // map[nodeKey]node
}

// Creates a new clean cache with the given size.
// The size is the maximum size of the cache in bytes.
func newCleanCache(size uint64) cleanCache {
	if size > uint64(math.MaxInt) {
		panic("cache size too large: uint64 to int conversion would overflow")
	}
	return cleanCache{
		cache: fastcache.New(int(size)),
	}
}

func (c *cleanCache) putNode(owner *felt.Address, path *trieutils.Path, isClass bool, blob []byte) {
	c.cache.Set(nodeKey(owner, path, isClass), blob)
}

func (c *cleanCache) getNode(owner *felt.Address, path *trieutils.Path, isClass bool) []byte {
	return c.cache.Get(nil, nodeKey(owner, path, isClass))
}

func (c *cleanCache) deleteNode(owner *felt.Address, path *trieutils.Path, isClass bool) {
	c.cache.Del(nodeKey(owner, path, isClass))
}

// key = owner (32 bytes) + path (20 bytes) + trie type (1 byte)
func nodeKey(owner *felt.Address, path *trieutils.Path, isClass bool) []byte {
	key := make([]byte, nodeCacheSize)
	ownerBytes := owner.Bytes()
	copy(key[:felt.Bytes], ownerBytes[:])
	copy(key[felt.Bytes:felt.Bytes+trieutils.PathSize], path.EncodedBytes())

	if isClass {
		key[nodeCacheSize-1] = byte(class)
	}

	return key
}
