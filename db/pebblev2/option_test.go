package pebblev2_test

import (
	"runtime"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/db/pebblev2"
	"github.com/NethermindEth/juno/utils"
	pebbledb "github.com/cockroachdb/pebble/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testCacheSizeMB    = 2048
	testMemtableSizeMB = 1024
	testMemtableCount  = 4
	testMaxOpenFiles   = 200
)

func TestOptions(t *testing.T) {
	options := []pebblev2.Option{
		pebblev2.WithCacheSize(testCacheSizeMB),
		pebblev2.WithMaxOpenFiles(testMaxOpenFiles),
		pebblev2.WithLogger(true),
		pebblev2.WithMemtableSize(testMemtableSizeMB),
		pebblev2.WithMemtableCount(testMemtableCount),
	}

	opt := pebbledb.Options{}
	for _, option := range options {
		require.NoError(t, option(&opt))
	}

	assert.Equal(t, int64(testCacheSizeMB*1024*1024), opt.Cache.MaxSize())
	assert.Equal(t, uint64(testMemtableSizeMB*1024*1024), opt.MemTableSize)
	assert.Equal(t, testMaxOpenFiles, opt.MaxOpenFiles)
	assert.Equal(t, testMemtableCount, opt.MemTableStopWritesThreshold)
	assert.NotNil(t, opt.Logger)
	assert.IsType(t, &utils.ZapLogger{}, opt.Logger)
}

func TestWithCompression(t *testing.T) {
	tests := []struct {
		compression   string
		expectedName  string
		expectedLevel uint8
		expectedErr   string
	}{
		{
			compression:   "snappy",
			expectedName:  "Snappy",
			expectedLevel: 0,
		},
		{
			compression:   "zstd",
			expectedName:  "ZSTD",
			expectedLevel: 3,
		},
		{
			compression:   "minlz",
			expectedName:  "MinLZ",
			expectedLevel: 1,
		},
		{
			compression:   "zstd1",
			expectedName:  "ZSTD",
			expectedLevel: 1,
		},
		{
			compression: "invalid",
			expectedErr: "unknown compression profile",
		},
	}

	for _, tt := range tests {
		for _, compression := range []string{tt.compression, strings.ToUpper(tt.compression)} {
			t.Run(compression, func(t *testing.T) {
				opt := pebbledb.Options{}
				err := pebblev2.WithCompression(compression)(&opt)

				if tt.expectedErr != "" {
					require.Error(t, err)
					assert.Contains(t, err.Error(), tt.expectedErr)
					return
				}

				require.NoError(t, err)
				for i := range opt.Levels {
					require.NotNil(t, opt.Levels[i].Compression)
					profile := opt.Levels[i].Compression()
					require.NotNil(t, profile)
					assert.Equal(t, tt.expectedName, profile.Name)
					assert.Equal(t, tt.expectedName, profile.DataBlocks.Algorithm.String())
					assert.Equal(t, tt.expectedLevel, profile.DataBlocks.Level)
					assert.Equal(t, tt.expectedName, profile.ValueBlocks.Algorithm.String())
					assert.Equal(t, tt.expectedLevel, profile.ValueBlocks.Level)
					assert.Equal(t, tt.expectedName, profile.OtherBlocks.Algorithm.String())
					assert.Equal(t, tt.expectedLevel, profile.OtherBlocks.Level)
				}
			})
		}
	}
}

func TestWithCompactionConcurrency(t *testing.T) {
	tests := []struct {
		name          string
		concurrency   string
		expectedLower int
		expectedUpper int
		expectedErr   string
	}{
		{
			name:          "empty string uses defaults",
			concurrency:   "",
			expectedLower: 1,
			expectedUpper: runtime.GOMAXPROCS(0) / 2,
		},
		{
			name:          "single value sets upper with lower=1",
			concurrency:   "4",
			expectedLower: 1,
			expectedUpper: 4,
		},
		{
			name:          "two values set lower and upper",
			concurrency:   "2,8",
			expectedLower: 2,
			expectedUpper: 8,
		},
		{
			name:          "whitespace is trimmed",
			concurrency:   " 3 , 6 ",
			expectedLower: 3,
			expectedUpper: 6,
		},
		{
			name:        "invalid single value",
			concurrency: "abc",
			expectedErr: "invalid compaction concurrency upper value",
		},
		{
			name:        "invalid lower value",
			concurrency: "abc,4",
			expectedErr: "invalid compaction concurrency lower value",
		},
		{
			name:        "invalid upper value",
			concurrency: "2,abc",
			expectedErr: "invalid compaction concurrency upper value",
		},
		{
			name:        "too many values",
			concurrency: "1,2,3",
			expectedErr: "invalid compaction concurrency format",
		},
		{
			name:        "lower bound less than 1",
			concurrency: "0,4",
			expectedErr: "compaction concurrency lower bound must be >= 1",
		},
		{
			name:        "upper less than lower",
			concurrency: "5,3",
			expectedErr: "compaction concurrency upper bound (3) must be >= lower bound (5)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opt := pebbledb.Options{}
			err := pebblev2.WithCompactionConcurrency(tt.concurrency)(&opt)

			if tt.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, opt.CompactionConcurrencyRange)
			lower, upper := opt.CompactionConcurrencyRange()
			assert.Equal(t, tt.expectedLower, lower)
			assert.Equal(t, tt.expectedUpper, upper)
		})
	}
}
