package pebble_test

import (
	"testing"

	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/utils"
	pebbledb "github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testCacheSizeMB  = 2048
	testMaxOpenFiles = 200
)

func TestOptions(t *testing.T) {
	options := []pebble.Option{
		pebble.WithCacheSize(testCacheSizeMB),
		pebble.WithMaxOpenFiles(testMaxOpenFiles),
		pebble.WithLogger(true),
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
