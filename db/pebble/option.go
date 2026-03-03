package pebble

import (
	"github.com/NethermindEth/juno/utils"
	"github.com/cockroachdb/pebble"
)

const (
	// minCache is the minimum amount of memory in megabytes to allocate to pebble
	// read and write caching. This is also pebble's default value.
	minCacheSizeMB = 8
)

type Option = func(*pebble.Options) error

func WithCacheSize(cacheSizeMB uint) Option {
	cacheSizeMB = max(cacheSizeMB, minCacheSizeMB)
	return func(opts *pebble.Options) error {
		opts.Cache = pebble.NewCache(int64(cacheSizeMB * utils.Megabyte))
		return nil
	}
}

func WithMaxOpenFiles(maxOpenFiles int) Option {
	return func(opts *pebble.Options) error {
		opts.MaxOpenFiles = maxOpenFiles
		return nil
	}
}

func WithLogger(logger pebble.Logger) Option {
	return func(opts *pebble.Options) error {
		opts.Logger = logger
		return nil
	}
}
