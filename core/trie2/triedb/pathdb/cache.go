package pathdb

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/VictoriaMetrics/fastcache"
)

var nodeCacheSize = ownerSize + trieutils.PathSize + 1

type cleanCache struct {
	cache *fastcache.Cache
}

func newCleanCache(size int) *cleanCache {
	return &cleanCache{cache: fastcache.New(size)}
}

func (c *cleanCache) putNode(owner felt.Felt, path trieutils.Path, isClass bool, blob []byte) {
	c.cache.Set(nodeKey(owner, path, isClass), blob)
}

func (c *cleanCache) getNode(owner felt.Felt, path trieutils.Path, isClass bool) []byte {
	return c.cache.Get(nil, nodeKey(owner, path, isClass))
}

func (c *cleanCache) deleteNode(owner felt.Felt, path trieutils.Path, isClass bool) {
	c.cache.Del(nodeKey(owner, path, isClass))
}

func nodeKey(owner felt.Felt, path trieutils.Path, isClass bool) []byte {
	key := make([]byte, nodeCacheSize)
	ownerBytes := owner.Bytes()
	copy(key[:felt.Bytes], ownerBytes[:])
	copy(key[felt.Bytes:felt.Bytes+trieutils.PathSize], path.EncodedBytes())

	if isClass {
		key[nodeCacheSize-1] = 1
	}

	return key
}
