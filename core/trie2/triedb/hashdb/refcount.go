package hashdb

import (
	"sync"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"
)

type RefCountCache struct {
	dirties map[common.Hash]*cachedNode
	oldest  common.Hash
	newest  common.Hash

	hits   uint64
	misses uint64

	lock sync.RWMutex
}

type cachedNode struct {
	blob      []byte
	parents   uint32
	external  map[common.Hash]struct{}
	flushPrev common.Hash
	flushNext common.Hash
}

func NewRefCountCache() *RefCountCache {
	return &RefCountCache{
		dirties: make(map[common.Hash]*cachedNode),
	}
}

func (c *RefCountCache) Get(key []byte) ([]byte, bool) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	hash := common.BytesToHash(key)
	node, ok := c.dirties[hash]
	if !ok {
		atomic.AddUint64(&c.misses, 1)
		return nil, false
	}

	atomic.AddUint64(&c.hits, 1)
	return node.blob, true
}

func (c *RefCountCache) Set(key []byte, value []byte) bool {
	c.lock.Lock()
	defer c.lock.Unlock()

	hash := common.BytesToHash(key)

	if _, ok := c.dirties[hash]; ok {
		return false
	}

	entry := &cachedNode{
		blob:      value,
		flushPrev: c.newest,
	}

	if c.oldest == (common.Hash{}) {
		c.oldest, c.newest = hash, hash
	} else {
		c.dirties[c.newest].flushNext, c.newest = hash, hash
	}

	c.dirties[hash] = entry
	return true
}

func (c *RefCountCache) Remove(key []byte) bool {
	c.lock.Lock()
	defer c.lock.Unlock()

	hash := common.BytesToHash(key)
	node, ok := c.dirties[hash]
	if !ok {
		return false
	}

	switch hash {
	case c.oldest:
		c.oldest = node.flushNext
		if node.flushNext != (common.Hash{}) {
			c.dirties[node.flushNext].flushPrev = common.Hash{}
		}
	case c.newest:
		c.newest = node.flushPrev
		if node.flushPrev != (common.Hash{}) {
			c.dirties[node.flushPrev].flushNext = common.Hash{}
		}
	default:
		c.dirties[node.flushPrev].flushNext = node.flushNext
		c.dirties[node.flushNext].flushPrev = node.flushPrev
	}

	delete(c.dirties, hash)
	return true
}

func (c *RefCountCache) Hits() uint64 {
	return atomic.LoadUint64(&c.hits)
}

func (c *RefCountCache) HitRate() float64 {
	hits := atomic.LoadUint64(&c.hits)
	misses := atomic.LoadUint64(&c.misses)
	total := hits + misses
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total)
}

func (c *RefCountCache) Len() int {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return len(c.dirties)
}

func (c *RefCountCache) GetOldest() ([]byte, []byte, bool) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	if c.oldest == (common.Hash{}) {
		return nil, nil, false
	}

	node := c.dirties[c.oldest]
	return c.oldest[:], node.blob, true
}

func (c *RefCountCache) RemoveOldest() bool {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.oldest == (common.Hash{}) {
		return false
	}

	oldest := c.oldest
	node := c.dirties[oldest]

	c.oldest = node.flushNext
	if node.flushNext != (common.Hash{}) {
		c.dirties[node.flushNext].flushPrev = common.Hash{}
	}

	delete(c.dirties, oldest)
	return true
}

func (c *RefCountCache) Reference(child common.Hash, parent common.Hash) {
	c.lock.Lock()
	defer c.lock.Unlock()

	node, ok := c.dirties[child]
	if !ok {
		return
	}

	if parent == (common.Hash{}) {
		node.parents += 1
		return
	}

	if c.dirties[parent].external == nil {
		c.dirties[parent].external = make(map[common.Hash]struct{})
	}
	if _, ok := c.dirties[parent].external[child]; ok {
		return
	}

	node.parents++
	c.dirties[parent].external[child] = struct{}{}
}

func (c *RefCountCache) Dereference(root common.Hash) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if root == (common.Hash{}) {
		return
	}

	c.dereference(root)
}

func (c *RefCountCache) dereference(hash common.Hash) {
	node, ok := c.dirties[hash]
	if !ok {
		return
	}

	if node.parents > 0 {
		node.parents--
	}

	if node.parents == 0 {
		switch hash {
		case c.oldest:
			c.oldest = node.flushNext
			if node.flushNext != (common.Hash{}) {
				c.dirties[node.flushNext].flushPrev = common.Hash{}
			}
		case c.newest:
			c.newest = node.flushPrev
			if node.flushPrev != (common.Hash{}) {
				c.dirties[node.flushPrev].flushNext = common.Hash{}
			}
		default:
			c.dirties[node.flushPrev].flushNext = node.flushNext
			c.dirties[node.flushNext].flushPrev = node.flushPrev
		}

		for child := range node.external {
			c.dereference(child)
		}

		delete(c.dirties, hash)
	}
}
