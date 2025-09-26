package utils

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func HexToUint64(t testing.TB, hexStr string) uint64 {
	t.Helper()

	if hexStr[:2] == "0x" {
		hexStr = hexStr[2:]
	}
	x, err := strconv.ParseUint(hexStr, 16, 64)
	require.NoError(t, err)
	return x
}
