package utils

import (
	"strconv"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/require"
)

func HexTo[T ~[4]uint64](t testing.TB, hex string) *T {
	t.Helper()

	f, err := new(felt.Felt).SetString(hex)
	require.NoError(t, err)
	x := T(*f)
	return &x
}

func HexToFelt(t testing.TB, hex string) *felt.Felt {
	t.Helper()
	return HexTo[felt.Felt](t, hex)
}

func HexToUint64(t testing.TB, hexStr string) uint64 {
	t.Helper()

	if hexStr[:2] == "0x" {
		hexStr = hexStr[2:]
	}
	x, err := strconv.ParseUint(hexStr, 16, 64)
	require.NoError(t, err)
	return x
}
