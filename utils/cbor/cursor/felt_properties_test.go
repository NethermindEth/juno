package cursor_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/cursor"
	"github.com/stretchr/testify/require"
)

// Generate valid felt encodings directly so every fuzz input exercises limb
// contents, suffix preservation and the boundary of the nesting limit.
func FuzzLimbsWide(f *testing.F) {
	for _, depth := range []byte{0, 31, 32} {
		f.Add(uint64(0), uint64(0), uint64(0), uint64(0), depth, []byte{})
		f.Add(^uint64(0), uint64(1), uint64(0x0102030405060708), uint64(1)<<63, depth, []byte{0xf6, 0xff})
	}
	f.Fuzz(func(t *testing.T, a, b, c, d uint64, depth byte, suffix []byte) {
		depth %= 33
		want := [cursor.Limbs]uint64{a, b, c, d}
		input := bytes.Repeat([]byte{0x81}, int(depth))
		input = append(input, 0x84)
		for _, word := range want {
			input = binary.BigEndian.AppendUint64(append(input, 0x1b), word)
		}
		input = append(input, suffix...)
		before := bytes.Clone(input)
		r := cursor.NewCursor(input)
		for range depth {
			n, err := r.ArrayLen()
			require.NoError(t, err)
			require.Equal(t, 1, n)
		}
		got := [cursor.Limbs]uint64{9, 8, 7, 6}
		if depth == 32 {
			for range 2 {
				err := r.Limbs(&got)
				require.ErrorIs(t, err, cursor.ErrDepth)
				var detail *cursor.DecodeError
				require.ErrorAs(t, err, &detail)
				require.Equal(t, int(depth), detail.Offset)
				require.Equal(t, int(depth), r.Pos())
				require.Equal(t, [cursor.Limbs]uint64{9, 8, 7, 6}, got)
			}
			// A failed attempt must preserve depth as well as position. Leaving
			// one enclosing array makes the same four-limb value readable.
			r.Leave()
		}
		require.NoError(t, r.Limbs(&got))
		require.Equal(t, want, got)
		require.Equal(t, int(depth)+37, r.Pos())
		require.True(t, bytes.Equal(suffix, r.Remaining()))
		require.Equal(t, before, input)
	})
}
