package main

import (
	"context"
	"sync"

	"golang.org/x/sync/singleflight"
)

// sierraCache memoizes per class hash whether the class is Sierra, sparing
// repeat getClass downloads; a class's content is immutable, so verdicts
// never expire. Errors are not cached.
type sierraCache struct {
	group    singleflight.Group
	mu       sync.RWMutex
	verdicts map[string]bool
}

func newSierraCache() *sierraCache {
	return &sierraCache{verdicts: make(map[string]bool)}
}

func (c *sierraCache) isSierra(
	ctx context.Context,
	client *rpcClient,
	blockNumber uint64,
	classHash string,
) (bool, error) {
	if verdict, ok := c.get(classHash); ok {
		return verdict, nil
	}
	result, err, _ := c.group.Do(classHash, func() (any, error) {
		class, err := client.classAt(ctx, blockNumber, classHash)
		if err != nil {
			return false, err
		}
		isSierra := len(class.SierraProgram) > 0
		c.set(classHash, isSierra)
		return isSierra, nil
	})
	if err != nil {
		return false, err
	}
	return result.(bool), nil
}

func (c *sierraCache) get(classHash string) (verdict, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	verdict, ok = c.verdicts[classHash]
	return verdict, ok
}

func (c *sierraCache) set(classHash string, isSierra bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.verdicts[classHash] = isSierra
}
