package utils_test

import (
	"testing"

	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGzip64(t *testing.T) {
	bytes := []byte{0}
	expectedComBytes := "H4sIAAAAAAAA/2IABAAA//+N7wLSAQAAAA=="
	comBytes, err := utils.Gzip64Encode(bytes)
	require.NoError(t, err)
	assert.Equal(t, comBytes, expectedComBytes)

	decompBytes, err := utils.Gzip64Decode(comBytes)
	require.NoError(t, err)
	assert.Equal(t, bytes, decompBytes)
}
