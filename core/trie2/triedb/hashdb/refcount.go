package hashdb

import (
	"sync"
	"sync/atomic"

	"github.com/NethermindEth/juno/core/felt"
)

type refcountCachedNode struct {
	cachedNode
	flushPrev string
	flushNext string
}

type RefCountCache struct {
	dirties map[string]*refcountCachedNode
	oldest  string
	newest  string

	hits   uint64
	misses uint64

	lock sync.RWMutex
}

func NewRefCountCache() *RefCountCache {
	return &RefCountCache{
		dirties: make(map[string]*refcountCachedNode),
	}
}

func (c *RefCountCache) Get(key []byte) (cachedNode, bool) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	node, ok := c.dirties[string(key)]
	if !ok {
		atomic.AddUint64(&c.misses, 1)
		return cachedNode{}, false
	}

	atomic.AddUint64(&c.hits, 1)
	return node.cachedNode, true
}

func (c *RefCountCache) Set(key []byte, node cachedNode) bool {
	c.lock.Lock()
	defer c.lock.Unlock()

	hash := felt.Felt{}
	hash.SetBytes(key)

	if _, ok := c.dirties[string(key)]; ok {
		return false
	}

	entry := &refcountCachedNode{
		cachedNode: node,
		flushPrev:  string(c.newest),
	}

	// TODO: Fix this with actual trie traversal
	for childKey := range node.external {
		if child, ok := c.dirties[childKey]; !ok {
			child.parents++
		}
	}

	if c.oldest == "" {
		c.oldest, c.newest = string(key), string(key)
	} else {
		c.dirties[string(c.newest)].flushNext, c.newest = string(key), string(key)
	}

	c.dirties[string(key)] = entry
	return true
}

func (c *RefCountCache) Remove(key []byte) bool {
	c.lock.Lock()
	defer c.lock.Unlock()

	node, ok := c.dirties[string(key)]
	if !ok {
		return false
	}

	switch string(key) {
	case c.oldest:
		c.oldest = node.flushNext
		if node.flushNext != "" {
			c.dirties[node.flushNext].flushPrev = ""
		}
	case c.newest:
		c.newest = node.flushPrev
		if node.flushPrev != "" {
			c.dirties[node.flushPrev].flushNext = ""
		}
	default:
		c.dirties[node.flushPrev].flushNext = node.flushNext
		c.dirties[node.flushNext].flushPrev = node.flushPrev
	}

	delete(c.dirties, string(key))
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

func (c *RefCountCache) GetOldest() (key []byte, value cachedNode, ok bool) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	if c.oldest == "" {
		return nil, cachedNode{}, false
	}

	node := c.dirties[c.oldest]
	return []byte(c.oldest), node.cachedNode, true
}

func (c *RefCountCache) RemoveOldest() bool {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.oldest == "" {
		return false
	}

	oldest := c.oldest
	node := c.dirties[oldest]

	c.oldest = node.flushNext
	if node.flushNext != "" {
		c.dirties[node.flushNext].flushPrev = ""
	}

	delete(c.dirties, oldest)
	return true
}

func (c *RefCountCache) Reference(child []byte, parent []byte) {
	c.lock.Lock()
	defer c.lock.Unlock()

	node, ok := c.dirties[string(child)]
	if !ok {
		return
	}

	if parent == nil {
		node.parents += 1
		return
	}

	if c.dirties[string(parent)].external == nil {
		c.dirties[string(parent)].external = make(map[string]struct{})
	}
	if _, ok := c.dirties[string(parent)].external[string(child)]; ok {
		return
	}

	node.parents++
	c.dirties[string(parent)].external[string(child)] = struct{}{}
}

func (c *RefCountCache) Dereference(root []byte) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if root == nil {
		return
	}

	c.dereference(string(root))
}

func (c *RefCountCache) dereference(hash string) {
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
			if node.flushNext != "" {
				c.dirties[node.flushNext].flushPrev = ""
			}
		case c.newest:
			c.newest = node.flushPrev
			if node.flushPrev != "" {
				c.dirties[node.flushPrev].flushNext = ""
			}
		default:
			c.dirties[node.flushPrev].flushNext = node.flushNext
			c.dirties[node.flushNext].flushPrev = node.flushPrev
		}

		// TODO: Fix this with actual trie traversal
		for child := range node.external {
			c.dereference(child)
		}

		delete(c.dirties, hash)
	}
}
