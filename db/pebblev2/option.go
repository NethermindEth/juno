package pebblev2

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"

	"github.com/NethermindEth/juno/utils"
	"github.com/cockroachdb/pebble/v2"
	"github.com/cockroachdb/pebble/v2/sstable/block"
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

func WithLogger(colouredLogger bool) Option {
	return func(opts *pebble.Options) error {
		log := utils.NewLogLevel(utils.ERROR)
		dbLog, err := utils.NewZapLogger(log, colouredLogger)
		if err != nil {
			return fmt.Errorf("create DB logger: %w", err)
		}
		opts.Logger = dbLog
		return nil
	}
}

func WithCompression(compression *block.CompressionProfile) Option {
	return func(opts *pebble.Options) error {
		opts.ApplyCompressionSettings(func() pebble.DBCompressionSettings {
			return pebble.UniformDBCompressionSettings(compression)
		})
		return nil
	}
}

// WithCompactionConcurrency sets the compaction concurrency range.
// Format: "N" sets lower=1, upper=N; "M,N" sets lower=M, upper=N.
// Empty string uses the default (1, GOMAXPROCS/2).
func WithCompactionConcurrency(concurrency string) Option {
	return func(opts *pebble.Options) error {
		lower, upper := 1, runtime.GOMAXPROCS(0)/2
		if concurrency != "" {
			var err error
			lower, upper, err = parseCompactionConcurrency(concurrency)
			if err != nil {
				return err
			}
		}

		opts.CompactionConcurrencyRange = func() (int, int) {
			return lower, upper
		}
		return nil
	}
}

func WithMemtableSize(memtableSizeMB uint) Option {
	return func(opts *pebble.Options) error {
		opts.MemTableSize = uint64(memtableSizeMB) * utils.Megabyte
		return nil
	}
}

func parseCompactionConcurrency(s string) (lower, upper int, err error) {
	parts := strings.Split(s, ",")
	switch len(parts) {
	case 1:
		upper, err = strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return 0, 0, fmt.Errorf("invalid compaction concurrency upper value: %w", err)
		}
		lower = 1
	case 2:
		lower, err = strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return 0, 0, fmt.Errorf("invalid compaction concurrency lower value: %w", err)
		}
		upper, err = strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return 0, 0, fmt.Errorf("invalid compaction concurrency upper value: %w", err)
		}
	default:
		return 0, 0, fmt.Errorf("invalid compaction concurrency format: expected N or M,N, got %q", s)
	}

	if lower < 1 {
		return 0, 0, fmt.Errorf("compaction concurrency lower bound must be >= 1, got %d", lower)
	}
	if upper < lower {
		return 0, 0, fmt.Errorf(
			"compaction concurrency upper bound (%d) must be >= lower bound (%d)", upper, lower,
		)
	}

	return lower, upper, nil
}
