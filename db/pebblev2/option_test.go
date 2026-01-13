package pebblev2_test

import (
	"runtime"
	"testing"

	"github.com/NethermindEth/juno/db/pebblev2"
	"github.com/NethermindEth/juno/utils"
	pebbledb "github.com/cockroachdb/pebble/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testCacheSizeMB  = 2048
	testMaxOpenFiles = 200
)

func TestOptions(t *testing.T) {
	options := []pebblev2.Option{
		pebblev2.WithCacheSize(testCacheSizeMB),
		pebblev2.WithMaxOpenFiles(testMaxOpenFiles),
		pebblev2.WithLogger(true),
	}

	opt := pebbledb.Options{}
	for _, option := range options {
		require.NoError(t, option(&opt))
	}

	assert.Equal(t, opt.Cache.MaxSize(), int64(testCacheSizeMB*1024*1024))
	assert.Equal(t, opt.MaxOpenFiles, testMaxOpenFiles)
	assert.NotNil(t, opt.Logger)
	assert.IsType(t, &utils.ZapLogger{}, opt.Logger)
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
