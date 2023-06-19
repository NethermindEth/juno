package utils_test

import (
	"testing"

	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGzip64Encode(t *testing.T) {
	bytes := []byte{0}
	expectedComBytes := "H4sIAAAAAAAA/2IABAAA//+N7wLSAQAAAA=="
	comBytes, err := utils.Gzip64Encode(bytes)
	require.Nil(t, err)
	assert.Equal(t, comBytes, expectedComBytes)
}
