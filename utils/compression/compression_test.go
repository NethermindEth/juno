package compression_test

import (
	"testing"

	"github.com/NethermindEth/juno/utils/compression"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGzip64(t *testing.T) {
	bytes := []byte{0}
	expectedComBytes := "H4sIAAAAAAAA/2IABAAA//+N7wLSAQAAAA=="
	comBytes, err := compression.Gzip64Encode(bytes)
	require.NoError(t, err)
	assert.Equal(t, comBytes, expectedComBytes)

	decompBytes, err := compression.Gzip64Decode(comBytes)
	require.NoError(t, err)
	assert.Equal(t, bytes, decompBytes)
}

func FuzzGzip64(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		compressed, err := compression.Gzip64Encode(data)
		require.NoError(t, err)
		decompressed, err := compression.Gzip64Decode(compressed)
		require.NoError(t, err)
		assert.Equal(t, data, decompressed)
	})
}
