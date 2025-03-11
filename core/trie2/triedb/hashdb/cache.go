package hashdb

import (
	"bytes"
)

type CacheType int

const (
	CacheTypeFastCache CacheType = iota
	CacheTypeLRU
	CacheTypeRefCount
)

type Cache interface {
	Get(buf *bytes.Buffer, key []byte) bool
	Set(key []byte, value []byte)
	Delete(key []byte)
	Hits() uint64
	HitRate() float64
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
}

func NewCache(cacheType CacheType, size int) Cache {
	switch cacheType {
	case CacheTypeFastCache:
		return NewFastCache(size)
	case CacheTypeLRU:
		return NewLRUCache(size)
	default:
		return NewFastCache(size)
	}
}
