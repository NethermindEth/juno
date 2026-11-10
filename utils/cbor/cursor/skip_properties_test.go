package cursor_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/cursor"
	"github.com/stretchr/testify/require"
)

func TestSkipAtomicWidths(t *testing.T) {
	limits := [...]uint64{23, 255, 65535, 1<<32 - 1, ^uint64(0)}
	for width, limit := range limits {
		for _, major := range []byte{0x00, 0x20, 0xe0} {
			for _, value := range []uint64{0, 23, 24, 32, limit} {
				if value > limit {
					continue
				}
				t.Run(fmt.Sprintf("major%x/width%d/value%d", major, width, value), func(t *testing.T) {
					wire := appendTestCBORArgument(nil, major, value, byte(width))
					var wantErr error
					if major == 0xe0 && width == 1 && value < 32 {
						wantErr = cursor.ErrSyntax
					}
					checkSkipAtomic(t, wire, wantErr, len(wire))
				})
			}
		}
		for _, major := range []byte{0x40, 0x60} {
			for _, count := range []uint64{0, 1, 23, 24, 64} {
				if count > limit {
					continue
				}
				t.Run(fmt.Sprintf("major%x/width%d/count%d", major, width, count), func(t *testing.T) {
					wire := appendTestCBORArgument(nil, major, count, byte(width))
					// Skip deliberately accepts text containing invalid UTF-8.
					wire = append(wire, bytes.Repeat([]byte{0xff}, int(count))...)
					checkSkipAtomic(t, wire, nil, 0)
				})
			}
		}
	}
}

func TestSkipAtomicRejections(t *testing.T) {
	for value := 0; value < 32; value++ {
		t.Run(fmt.Sprintf("simple%d", value), func(t *testing.T) {
			checkSkipAtomic(t, []byte{0xf8, byte(value)}, cursor.ErrSyntax, 2)
		})
	}
	for width := byte(0); width < 5; width++ {
		for tag := uint64(0); tag <= 3; tag++ {
			t.Run(fmt.Sprintf("tag%d/width%d", tag, width), func(t *testing.T) {
				wire := appendTestCBORArgument(nil, 0xc0, tag, width)
				offset := len(wire)
				wire = append(wire, bytes.Repeat([]byte{0x1b}, 40)...)
				checkSkipAtomic(t, wire, cursor.ErrUnsupported, offset)
			})
		}
		for _, major := range []byte{0x40, 0x60} {
			t.Run(fmt.Sprintf("short%x/width%d", major, width), func(t *testing.T) {
				wire := appendTestCBORArgument(nil, major, 23, width)
				offset := len(wire)
				wire = append(wire, bytes.Repeat([]byte{0xff}, 22)...)
				checkSkipAtomic(t, wire, cursor.ErrTruncated, offset)
			})
		}
	}
	for _, major := range []byte{0x40, 0x60} {
		t.Run(fmt.Sprintf("maxLength%x", major), func(t *testing.T) {
			wire := appendTestCBORArgument(nil, major, ^uint64(0), 4)
			offset := len(wire)
			wire = append(wire, bytes.Repeat([]byte{0x1b}, 40)...)
			checkSkipAtomic(t, wire, cursor.ErrTruncated, offset)
		})
	}
	t.Run("eof", func(t *testing.T) { checkSkipAtomic(t, nil, cursor.ErrTruncated, 0) })
}

// Exercise the public cursor at nonzero offsets, including the maximum enclosing
// depth. Failure is retryable and retains the diagnostic's offset within the item.
func checkSkipAtomic(t *testing.T, wire []byte, wantErr error, errorOffset int) {
	t.Helper()
	for _, depth := range []int{0, 31, 32} {
		input := append(bytes.Repeat([]byte{0x81}, depth), 0, 1, 2)
		start := len(input)
		input = append(input, wire...)
		var suffix []byte
		if wantErr == nil {
			suffix = append([]byte{0x80}, bytes.Repeat([]byte{0x1b}, 40)...)
			input = append(input, suffix...)
		}
		before := bytes.Clone(input)
		r := cursor.NewCursor(input)
		for range depth {
			n, err := r.ArrayLen()
			require.NoError(t, err)
			require.Equal(t, 1, n)
		}
		for range 3 {
			_, err := r.Uint64()
			require.NoError(t, err)
		}
		if wantErr != nil {
			for range 2 {
				err := r.Skip()
				require.ErrorIs(t, err, wantErr)
				var detail *cursor.DecodeError
				require.ErrorAs(t, err, &detail)
				require.Equal(t, start+errorOffset, detail.Offset)
				require.Equal(t, start, r.Pos())
				require.True(t, bytes.Equal(wire, r.Remaining()))
			}
		} else {
			require.NoError(t, r.Skip())
			require.Equal(t, start+len(wire), r.Pos())
			require.Equal(t, suffix, r.Remaining())
			// Skipping an atom must preserve the enclosing depth. The next
			// empty array opens one more level and detects any lost level.
			if depth == 32 {
				require.ErrorIs(t, r.Skip(), cursor.ErrDepth)
				require.Equal(t, start+len(wire), r.Pos())
			} else {
				require.NoError(t, r.Skip())
				require.Equal(t, start+len(wire)+1, r.Pos())
			}
		}
		require.Equal(t, before, input)
	}
}
