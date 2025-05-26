package hashdb

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/VictoriaMetrics/fastcache"
)

type CleanCache struct {
	cache *fastcache.Cache
}

func NewCleanCache(size int) *CleanCache {
	if size <= 0 {
		return nil
	}
	return &CleanCache{
		cache: fastcache.New(size),
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
