package cursor_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/cursor"
	"github.com/stretchr/testify/require"
)

func TestUint64HeadClassification(t *testing.T) {
	for h := range 256 {
		head := byte(h)
		major, info := head>>5, head&31
		width := 0
		if info >= 24 && info <= 27 {
			width = 1 << (info - 24)
		}
		t.Run(fmt.Sprintf("%02x/complete", head), func(t *testing.T) {
			// Three preceding integers exercise a nonzero error offset. The final
			// null is a suffix that a successful unsigned read must leave intact.
			input := append([]byte{0, 1, 2, head}, make([]byte, width)...)
			input = append(input, 0xf6)
			before := bytes.Clone(input)
			c := cursor.NewCursor(input)
			for range 3 {
				_, err := c.Uint64()
				require.NoError(t, err)
			}
			var wantError error
			switch {
			case info == 31 && major >= 2 && major <= 5:
				wantError = cursor.ErrUnsupported
			case info >= 28:
				wantError = cursor.ErrSyntax
			case major == 6:
				wantError = cursor.ErrUnsupported
			case major != 0:
				wantError = cursor.ErrType
			}
			if wantError == nil {
				got, err := c.Uint64()
				require.NoError(t, err)
				want := uint64(0)
				if info < 24 {
					want = uint64(info)
				}
				require.Equal(t, want, got)
				require.Equal(t, []byte{0xf6}, c.Remaining())
			} else {
				for range 2 {
					got, err := c.Uint64()
					require.Zero(t, got)
					require.ErrorIs(t, err, wantError)
					var detail *cursor.DecodeError
					require.ErrorAs(t, err, &detail)
					require.Equal(t, 3, detail.Offset)
					require.Equal(t, 3, c.Pos())
					require.Equal(t, input[3:], c.Remaining())
				}
			}
			require.Equal(t, before, input)
		})
		// Every cut through an extended argument must report truncation,
		// including a head whose major type would otherwise be rejected.
		for available := range width {
			t.Run(fmt.Sprintf("%02x/payload=%d", head, available), func(t *testing.T) {
				input := append([]byte{0, 1, 2, head}, make([]byte, available)...)
				before := bytes.Clone(input)
				c := cursor.NewCursor(input)
				for range 3 {
					_, err := c.Uint64()
					require.NoError(t, err)
				}
				for range 2 {
					got, err := c.Uint64()
					require.Zero(t, got)
					require.ErrorIs(t, err, cursor.ErrTruncated)
					var detail *cursor.DecodeError
					require.ErrorAs(t, err, &detail)
					require.Equal(t, 3, detail.Offset)
					require.Equal(t, 3, c.Pos())
					require.Equal(t, input[3:], c.Remaining())
				}
				require.Equal(t, before, input)
			})
		}
	}
}
