package fast_test

import (
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/fast"
	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
	"github.com/stretchr/testify/require"
)

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := fxamacker.Marshal(v)
	require.NoError(t, err)
	return b
}

func TestBytesInto(t *testing.T) {
	r := fast.NewReader(mustMarshal(t, []byte{1, 2, 3}))
	var short [2]byte
	require.ErrorIs(t, r.BytesInto(short[:]), fast.ErrType)
	var long [4]byte
	require.ErrorIs(t, r.BytesInto(long[:]), fast.ErrType)
	require.Zero(t, r.Pos())

	var exact [3]byte
	require.NoError(t, r.BytesInto(exact[:]))
	require.Equal(t, [3]byte{1, 2, 3}, exact)
	require.Zero(t, r.Len())

	r = fast.NewReader(mustMarshal(t, "abc"))
	require.ErrorIs(t, r.BytesInto(exact[:]), fast.ErrType)
	require.Zero(t, r.Pos())
}
