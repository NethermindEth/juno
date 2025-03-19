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

type CleanCache interface {
	Get(buf *bytes.Buffer, key []byte) bool
	Set(key []byte, value []byte)
	Remove(key []byte)
	Hits() uint64
	HitRate() float64
}

type DirtyCache interface {
	Get(buf *bytes.Buffer, key []byte) bool
	Set(key []byte, value []byte) bool
	Remove(key []byte) bool
	Peek(key []byte) ([]byte, bool)
	Hits() uint64
	HitRate() float64
	Len() int
	GetOldest() ([]byte, []byte, bool)
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
