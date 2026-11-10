package cursor_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/cursor"
	"github.com/stretchr/testify/require"
)

func TestMalformedHeadsInFeltAndSkip(t *testing.T) {
	for h := range 256 {
		head := byte(h)
		if head&31 < 28 {
			continue
		}
		kind := cursor.ErrSyntax
		if head&31 == 31 && head>>5 >= 2 && head>>5 <= 5 {
			kind = cursor.ErrUnsupported
		}
		for _, tail := range []int{0, 40} {
			// Head errors take precedence over depth errors, even at depth 32.
			for _, depth := range []int{0, 31, 32} {
				t.Run(fmt.Sprintf("head%02x/tail%d/depth%d", head, tail, depth), func(t *testing.T) {
					input := append(bytes.Repeat([]byte{0x81}, depth), 0, 1, 2)
					start := len(input)
					input = append(input, head)
					input = append(input, bytes.Repeat([]byte{0x1b}, tail)...)
					before := bytes.Clone(input)
					r := openSkipCursor(t, input, depth)
					for range 2 {
						checkPayloadError(t, &r, r.Skip(), kind, start, input[start:])
						got := [cursor.Limbs]uint64{9, 8, 7, 6}
						checkPayloadError(t, &r, r.Limbs(&got), kind, start, input[start:])
						require.Equal(t, [cursor.Limbs]uint64{9, 8, 7, 6}, got)
					}
					require.Equal(t, before, input)
				})
			}
			// Before four items the bad head is a limb; afterwards it is an
			// ignored value. Both paths must retain the inner diagnostic.
			for prefix := range 8 {
				for width := byte(0); width < 5; width++ {
					for _, depth := range []int{0, 31} {
						t.Run(fmt.Sprintf("head%02x/tail%d/prefix%d/width%d/depth%d", head, tail, prefix, width, depth), func(t *testing.T) {
							input := append(bytes.Repeat([]byte{0x81}, depth), 0, 1, 2)
							start := len(input)
							input = appendTestCBORArgument(input, 0x80, uint64(prefix+1), width)
							for i := range prefix {
								if i < cursor.Limbs {
									input = appendTestCBORArgument(input, 0, uint64(i+1), (width+byte(i))%5)
								} else if depth == 0 {
									input = append(input, 0xa1, 0x00, 0x81, 0xf6)
								} else {
									input = append(input, 0xf6)
								}
							}
							badOffset := len(input)
							input = append(input, head)
							input = append(input, bytes.Repeat([]byte{0x1b}, tail)...)
							before := bytes.Clone(input)
							r := openSkipCursor(t, input, depth)
							old := [cursor.Limbs]uint64{9, 8, 7, 6}
							got := old
							for range 2 {
								err := r.Limbs(&got)
								require.ErrorIs(t, err, kind)
								var detail *cursor.DecodeError
								require.ErrorAs(t, err, &detail)
								require.Equal(t, badOffset, detail.Offset)
								require.Equal(t, start, r.Pos())
								require.Equal(t, input[start:], r.Remaining())
								require.Equal(t, old, got)
							}
							require.Equal(t, before, input)
						})
					}
				}
			}
		}
	}
}
