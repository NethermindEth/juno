package utils

import (
	"strconv"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/require"
)

type BenchmarkTesting interface {
	Errorf(format string, args ...interface{})
	FailNow()
	Logf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
	Cleanup(func())
}

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
