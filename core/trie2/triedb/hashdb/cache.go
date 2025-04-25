package hashdb

type CacheType int

const (
	CacheTypeFastCache CacheType = iota
	CacheTypeLRU
	CacheTypeRefCount
)

type cachedNode struct {
	blob []byte
}

type CleanCache interface {
	Get(key []byte) ([]byte, bool)
	Set(key []byte, value []byte)
	Remove(key []byte) bool
	Hits() uint64
	HitRate() float64
}

type DirtyCache interface {
	Get(key []byte) (cachedNode, bool)
	Set(key []byte, value cachedNode) bool
	Remove(key []byte) bool
	Hits() uint64
	HitRate() float64
	Len() int
	GetOldest() (key []byte, value cachedNode, ok bool)
	RemoveOldest() bool
}

type Config struct {
	DirtyCacheSize int
	CleanCacheSize int

	DirtyCacheType CacheType
	CleanCacheType CacheType
}

var DefaultConfig = &Config{
	DirtyCacheSize: 1024 * 1024 * 64,
	CleanCacheSize: 1024 * 1024 * 64,

	DirtyCacheType: CacheTypeLRU,
	CleanCacheType: CacheTypeFastCache,
}

func NewDirtyCache(cacheType CacheType, size int) DirtyCache {
	switch cacheType {
	case CacheTypeLRU:
		return NewLRUCache(size)
	default:
		return NewLRUCache(size)
	}
}

func NewCleanCache(cacheType CacheType, size int) CleanCache {
	switch cacheType {
	case CacheTypeFastCache:
		return NewFastCache(size)
	default:
		return NewFastCache(size)
	}
}
