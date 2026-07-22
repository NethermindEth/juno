package felt_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/require"
)

func TestIsZero(t *testing.T) {
	var val f
	require.True(t, felt.IsZero(&val))

	val[0] = 1
	require.False(t, felt.IsZero(&val))
}

func TestEqual(t *testing.T) {
	base := [4]uint64{1, 2, 3, 4}
	v1 := f(base)
	v2 := f(base)
	require.True(t, felt.Equal(&v1, &v2))

	v2[0] += 100
	require.False(t, felt.Equal(&v1, &v2))
}
