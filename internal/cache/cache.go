package cache

// Cache represents an on-memory cache, regardless of cache policy.
type Cache interface {
	Put(key []byte, value []byte)
	Get(key []byte) []byte
	Count() int
	Capacity() int
	Clear()
}
