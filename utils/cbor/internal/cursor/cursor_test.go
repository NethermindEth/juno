package cursor_test

import (
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/internal/cursor"
	"github.com/stretchr/testify/require"
)

func TestBytesInto(t *testing.T) {
	c := cursor.NewCursor([]byte{0x43, 1, 2, 3}) // a byte string of 1, 2, 3
	var short [2]byte
	require.ErrorIs(t, c.BytesInto(short[:]), cursor.ErrType)
	var long [4]byte
	require.ErrorIs(t, c.BytesInto(long[:]), cursor.ErrType)
	require.Zero(t, c.Pos())

	var exact [3]byte
	require.NoError(t, c.BytesInto(exact[:]))
	require.Equal(t, [3]byte{1, 2, 3}, exact)
	require.Zero(t, c.Len())

	c = cursor.NewCursor([]byte{0x63, 'a', 'b', 'c'}) // a text string
	require.ErrorIs(t, c.BytesInto(exact[:]), cursor.ErrType)
	require.Zero(t, c.Pos())
}

// Tags 0-3 hold a time or a big integer, whose content Skip does not check, so it refuses them.
func TestSkipRefusesContentTags(t *testing.T) {
	for tag := byte(0xc0); tag <= 0xc4; tag++ {
		c := cursor.NewCursor([]byte{tag, 0x01})
		err := c.Skip()
		if tag <= 0xc3 {
			require.ErrorIs(t, err, cursor.ErrUnsupported)
			require.Zero(t, c.Pos())
		} else {
			require.NoError(t, err)
		}
	}
}
