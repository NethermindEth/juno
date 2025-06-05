package hashdb

import (
	"math"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/VictoriaMetrics/fastcache"
)

type CleanCache struct {
	cache *fastcache.Cache
}

func NewCleanCache(size uint64) CleanCache {
	if size > uint64(math.MaxInt) {
		panic("cache size too large: uint64 to int conversion would overflow")
	}
	return CleanCache{
		cache: fastcache.New(int(size)),
	}
}

func (c *CleanCache) getNode(path *trieutils.Path, hash *felt.Felt) []byte {
	key := nodeKey(path, hash)
	value := c.cache.Get(nil, key)

	return value
}

func (c *CleanCache) putNode(path *trieutils.Path, hash *felt.Felt, value []byte) {
	key := nodeKey(path, hash)
	c.cache.Set(key, value)
}
