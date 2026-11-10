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

func TestSkipTaggedErrors(t *testing.T) {
	leaves := []struct {
		name   string
		wire   []byte
		kind   error
		offset int
	}{
		{name: "eof", kind: cursor.ErrTruncated},
		{name: "reserved", wire: []byte{0x1c}, kind: cursor.ErrSyntax},
		{name: "break", wire: []byte{0xff}, kind: cursor.ErrSyntax},
		{name: "indefinite", wire: []byte{0x9f}, kind: cursor.ErrUnsupported},
		{name: "badTagHead", wire: []byte{0xdc}, kind: cursor.ErrSyntax},
		{name: "shortUint", wire: []byte{0x1b, 1, 2}, kind: cursor.ErrTruncated},
		{name: "shortTag", wire: []byte{0xdb, 1, 2}, kind: cursor.ErrTruncated},
		{name: "simple", wire: []byte{0xf8, 24}, kind: cursor.ErrSyntax, offset: 2},
		{name: "bytes", wire: []byte{0x42, 1}, kind: cursor.ErrTruncated, offset: 1},
		{name: "text", wire: []byte{0x62, 1}, kind: cursor.ErrTruncated, offset: 1},
		{name: "unsupportedTag", wire: []byte{0xc0}, kind: cursor.ErrUnsupported, offset: 1},
	}
	for _, depth := range []int{0, 1, 15, 31, 32} {
		for count := range 35 {
			for _, leaf := range leaves {
				t.Run(fmt.Sprintf("%s/depth%d/tags%d", leaf.name, depth, count), func(t *testing.T) {
					input := append(bytes.Repeat([]byte{0x81}, depth), 0, 1, 2)
					start := len(input)
					ends := make([]int, count)
					for i := range count {
						// Alternate preferred and non-preferred tag encodings.
						input = appendTestCBORArgument(input, 0xc0, uint64(4+i%20), byte(i%5))
						ends[i] = len(input)
					}
					wantOffset := len(input) + leaf.offset
					wantErr := leaf.kind
					input = append(input, leaf.wire...)
					// A tag before a non-tag (or EOF) is free. A tag before
					// another tag charges depth before that child's head is read.
					charged := count - 1
					if len(leaf.wire) > 0 && leaf.wire[0]&0xe0 == 0xc0 {
						charged = count
					}
					if depth+charged > 32 {
						wantErr = cursor.ErrDepth
						wantOffset = ends[32-depth]
					}
					before := bytes.Clone(input)
					r := openSkipCursor(t, input, depth)
					for range 2 {
						err := r.Skip()
						require.ErrorIs(t, err, wantErr)
						var detail *cursor.DecodeError
						require.ErrorAs(t, err, &detail)
						require.Equal(t, wantOffset, detail.Offset)
						require.Equal(t, start, r.Pos())
						require.Equal(t, input[start:], r.Remaining())
					}
					require.Equal(t, before, input)
				})
			}
		}
	}
}
