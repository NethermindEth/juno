package cursor_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
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

// Exercise short arrays and independently chosen, possibly non-preferred widths
// for the array head and each limb. The expected words come from the generated
// values, independently of any CBOR decoder.
func FuzzLimbsWidths(f *testing.F) {
	for shape := byte(0); shape < 25; shape++ {
		f.Add(uint64(23), uint64(255), uint64(65535), ^uint64(0), uint16(0), shape, []byte{0xf6})
		f.Add(uint64(0), uint64(1), uint64(0x0102030405060708), uint64(1)<<63, uint16(586), shape, []byte{})
	}
	f.Fuzz(func(t *testing.T, a, b, c, d uint64, widths uint16, shape byte, suffix []byte) {
		count, arrayWidth := shape%5, (shape/5)%5
		input := appendTestCBORArgument([]byte{0, 1, 2}, 0x80, uint64(count), arrayWidth)
		values := [cursor.Limbs]uint64{a, b, c, d}
		var want [cursor.Limbs]uint64
		for i := range count {
			width := byte(widths % 5)
			widths /= 5
			word := values[i]
			switch width {
			case 0:
				word %= 24
			case 1:
				word &= 0xff
			case 2:
				word &= 0xffff
			case 3:
				word &= 0xffffffff
			}
			want[i] = word
			input = appendTestCBORArgument(input, 0, word, width)
		}
		end := len(input)
		input = append(input, suffix...)
		before := bytes.Clone(input)
		r := cursor.NewCursor(input)
		for range 3 {
			_, err := r.Uint64()
			require.NoError(t, err)
		}
		got := [cursor.Limbs]uint64{9, 8, 7, 6}
		require.NoError(t, r.Limbs(&got))
		require.Equal(t, want, got)
		require.Equal(t, end, r.Pos())
		require.True(t, bytes.Equal(suffix, r.Remaining()))
		require.Equal(t, before, input)
	})
}

// Cover every width combination for zero through four limbs. The trailing 0x1b
// bytes resemble wide-integer heads, so shape checks must respect the actual
// encoded item boundaries. The shortest four-limb case straddles the 37-byte
// fast-path threshold with the two nonempty suffixes.
func TestLimbsWidthCombinations(t *testing.T) {
	values := [...]uint64{23, 0x1b, 0x1b1b, 0x1b1b1b1b, 0x1b1b1b1b1b1b1b1b}
	suffixes := [][]byte{nil, bytes.Repeat([]byte{0x1b}, 31), bytes.Repeat([]byte{0x1b}, 32)}
	combinations := 1
	for count := 0; count <= cursor.Limbs; count++ {
		for arrayWidth := byte(0); arrayWidth < 5; arrayWidth++ {
			for widths := 0; widths < combinations; widths++ {
				t.Run(fmt.Sprintf("count%d/array%d/widths%d", count, arrayWidth, widths), func(t *testing.T) {
					encoded := appendTestCBORArgument([]byte{0, 1, 2}, 0x80, uint64(count), arrayWidth)
					var want [cursor.Limbs]uint64
					choices := widths
					for i := range count {
						width := byte(choices % 5)
						choices /= 5
						want[i] = values[width]
						encoded = appendTestCBORArgument(encoded, 0, want[i], width)
					}
					for _, suffix := range suffixes {
						input := append(bytes.Clone(encoded), suffix...)
						before := bytes.Clone(input)
						r := cursor.NewCursor(input)
						for range 3 {
							_, err := r.Uint64()
							require.NoError(t, err)
						}
						got := [cursor.Limbs]uint64{9, 8, 7, 6}
						require.NoError(t, r.Limbs(&got))
						require.Equal(t, want, got)
						require.Equal(t, len(encoded), r.Pos())
						require.True(t, bytes.Equal(suffix, r.Remaining()))
						require.Equal(t, before, input)
					}
				})
			}
		}
		combinations *= 5
	}
}

func appendTestCBORArgument(input []byte, major byte, value uint64, width byte) []byte {
	switch width {
	case 0:
		return append(input, major|byte(value))
	case 1:
		return append(input, major|24, byte(value))
	case 2:
		return binary.BigEndian.AppendUint16(append(input, major|25), uint16(value))
	case 3:
		return binary.BigEndian.AppendUint32(append(input, major|26), uint32(value))
	default:
		return binary.BigEndian.AppendUint64(append(input, major|27), value)
	}
}

// Extra array elements need not be integers: they are ignored CBOR values, whose
// nested contents must be consumed without changing the four retained words.
func FuzzLimbsExtended(f *testing.F) {
	for _, shape := range []byte{0, 1, 7, 9, 23, 39} {
		for _, depth := range []byte{0, 30, 31} {
			f.Add(uint64(1), uint64(255), uint64(65535), ^uint64(0), uint16(586), shape, depth,
				[]byte{4, 0, 2, 8, 6, 0, 5, 0, 1}, []byte{0xf6, 0xff})
		}
	}
	f.Fuzz(func(t *testing.T, a, b, c, d uint64, widths uint16, shape, depth byte, choices, suffix []byte) {
		depth %= 32
		extra, arrayWidth := shape%8, (shape/8)%5
		cutSelector := int(widths)
		input := append(bytes.Repeat([]byte{0x81}, int(depth)), 0, 1, 2)
		start := len(input)
		input = appendTestCBORArgument(input, 0x80, uint64(cursor.Limbs)+uint64(extra), arrayWidth)
		values := [cursor.Limbs]uint64{a, b, c, d}
		var want [cursor.Limbs]uint64
		for i, word := range values {
			width := byte(widths % 5)
			widths /= 5
			switch width {
			case 0:
				word %= 24
			case 1:
				word &= 0xff
			case 2:
				word &= 0xffff
			case 3:
				word &= 0xffffffff
			}
			want[i] = word
			input = appendTestCBORArgument(input, 0, word, width)
		}
		g := skipGenerator{choices: choices, fuel: 64}
		for range extra {
			input = append(input, g.item(int(depth)+1, false)...)
		}
		end := len(input)
		input = append(input, suffix...)
		before := bytes.Clone(input)
		r := openSkipCursor(t, input, int(depth))
		got := [cursor.Limbs]uint64{9, 8, 7, 6}
		require.NoError(t, r.Limbs(&got))
		require.Equal(t, want, got)
		require.Equal(t, end, r.Pos())
		require.True(t, bytes.Equal(suffix, r.Remaining()))
		require.Equal(t, before, input)

		cut := start + cutSelector%(end-start)
		short := bytes.Clone(input[:cut])
		shortBefore := bytes.Clone(short)
		r = openSkipCursor(t, short, int(depth))
		old := [cursor.Limbs]uint64{9, 8, 7, 6}
		got = old
		for range 2 {
			require.ErrorIs(t, r.Limbs(&got), cursor.ErrTruncated)
			require.Equal(t, old, got)
			require.Equal(t, start, r.Pos())
		}
		require.Equal(t, shortBefore, short)
	})
}
