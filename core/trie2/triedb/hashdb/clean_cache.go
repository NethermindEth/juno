package hashdb

import (
	"math"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/VictoriaMetrics/fastcache"
)

type cleanCache struct {
	cache *fastcache.Cache
}

func newCleanCache(size uint64) cleanCache {
	if size > uint64(math.MaxInt) {
		panic("cache size too large: uint64 to int conversion would overflow")
	}
	return cleanCache{
		cache: fastcache.New(int(size)),
	}
}

func (c *cleanCache) getNode(path *trieutils.Path, hash *felt.Hash) []byte {
	key := nodeKey(path, hash)
	value := c.cache.Get(nil, key)

	return value
}

func (c *cleanCache) putNode(path *trieutils.Path, hash *felt.Hash, value []byte) {
	key := nodeKey(path, hash)
	c.cache.Set(key, value)
}
