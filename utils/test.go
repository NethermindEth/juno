package utils

import (
	"strconv"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/uint128"
	"github.com/stretchr/testify/require"
)

func HexToFelt(t testing.TB, hex string) *felt.Felt {
	t.Helper()
	f, err := new(felt.Felt).SetString(hex)
	require.NoError(t, err)
	return f
}

func HexToUint64(t testing.TB, hexStr string) uint64 {
	if hexStr[:2] == "0x" {
		hexStr = hexStr[2:]
	}
	x, err := strconv.ParseUint(hexStr, 16, 64)
	require.NoError(t, err)
	return x
}

func HexToUint128(t testing.TB, hexStr string) *uint128.Int {
	t.Helper()
	u, err := new(uint128.Int).SetString(hexStr)
	require.NoError(t, err)
	return u
}
