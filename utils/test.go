package utils

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/require"
)

func HexToFelt(t testing.TB, hex string) *felt.Felt {
	t.Helper()
	f, err := new(felt.Felt).SetString(hex)
	require.NoError(t, err)
	return f
}

func NoErr[T any](v T, err error) func(*testing.T) T {
	return func(t *testing.T) T {
		t.Helper()
		require.NoError(t, err)

		return v
	}
}
