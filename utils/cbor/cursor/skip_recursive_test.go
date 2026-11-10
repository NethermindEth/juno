package cursor_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/cursor"
	"github.com/stretchr/testify/require"
)

// Generate complete values, rather than hoping byte mutations produce nested
// arrays, maps and tags. Expected consumption is the generated wire's length.
func FuzzSkipRecursive(f *testing.F) {
	for _, seed := range [][]byte{
		{}, {4, 0, 3, 8}, {5, 4, 2, 6, 4, 4, 1, 8},
		bytes.Repeat([]byte{6}, 100), bytes.Repeat([]byte{8, 0x1b}, 100),
	} {
		for _, depth := range []byte{0, 30, 31, 32} {
			f.Add(seed, depth, []byte{0xf6, 0x1b, 0xff})
		}
	}
	f.Fuzz(func(t *testing.T, choices []byte, depth byte, suffix []byte) {
		depth %= 33
		g := skipGenerator{choices: choices, fuel: 64}
		wire := g.item(int(depth), false)
		input := append(bytes.Repeat([]byte{0x81}, int(depth)), 0, 1, 2)
		start := len(input)
		input = append(input, wire...)
		input = append(input, suffix...)
		before := bytes.Clone(input)
		r := openSkipCursor(t, input, int(depth))
		require.NoError(t, r.Skip())
		require.Equal(t, start+len(wire), r.Pos())
		require.True(t, bytes.Equal(suffix, r.Remaining()))
		require.Equal(t, before, input)

		// Every proper prefix of a generated complete item lacks bytes. Sample
		// one cut per fuzz input, without a suffix that could complete it.
		cut := int(g.byte()) % len(wire)
		short := bytes.Clone(input[:start+cut])
		shortBefore := bytes.Clone(short)
		r = openSkipCursor(t, short, int(depth))
		for range 2 {
			require.ErrorIs(t, r.Skip(), cursor.ErrTruncated)
			require.Equal(t, start, r.Pos())
		}
		require.Equal(t, shortBefore, short)
	})
}

func TestSkipRecursiveDepthBoundaries(t *testing.T) {
	wide := []byte{0x84}
	for range cursor.Limbs {
		wide = binary.BigEndian.AppendUint64(append(wide, 0x1b), 0x1b1b1b1b1b1b1b1b)
	}
	for _, tc := range []struct {
		name string
		wire []byte
	}{
		{"arrays", append(bytes.Repeat([]byte{0x81}, 32), 0)},
		{"maps", append(bytes.Repeat([]byte{0xa1, 0}, 32), 0)},
		{"tags", append(bytes.Repeat([]byte{0xc4}, 33), 0)},
		{"tagsAroundArrays", append(bytes.Repeat([]byte{0xc4, 0x81}, 32), 0)},
		{"wideAtLastLevel", append(bytes.Repeat([]byte{0x81}, 31), wide...)},
		{"mixedWideChildren", append(append([]byte{0x83, 0}, wide...), 0xf6)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := append(bytes.Clone(tc.wire), 0xf6)
			r := cursor.NewCursor(input)
			require.NoError(t, r.Skip())
			require.Equal(t, len(tc.wire), r.Pos())
			require.Equal(t, []byte{0xf6}, r.Remaining())
			for cut := range len(tc.wire) {
				r = cursor.NewCursor(tc.wire[:cut])
				require.ErrorIs(t, r.Skip(), cursor.ErrTruncated, "cut %d", cut)
				require.Zero(t, r.Pos())
			}
		})
	}
}

func openSkipCursor(t *testing.T, input []byte, depth int) cursor.Cursor {
	t.Helper()
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
	return r
}

// The fuel bounds generation work; depth follows the application contract.
// parentTag defers charging a tag until its content's type is known.
type skipGenerator struct {
	choices []byte
	pos     int
	fuel    int
}

func (g *skipGenerator) byte() byte {
	if g.pos >= len(g.choices) {
		return 0
	}
	b := g.choices[g.pos]
	g.pos++
	return b
}

func (g *skipGenerator) item(depth int, parentTag bool) []byte {
	if g.fuel == 0 {
		return []byte{0}
	}
	g.fuel--
	kind := g.byte() % 9
	width := g.byte() % 5
	if kind == 6 && parentTag {
		if depth == 32 {
			kind = 0
		} else {
			depth++
		}
	}
	if depth == 32 && (kind == 4 || kind == 5 || kind == 8) {
		kind = 0
	}
	switch kind {
	case 2, 3:
		n := int(g.byte() % 8)
		out := appendTestCBORArgument(nil, 0x40+(kind-2)*0x20, uint64(n), width)
		for range n {
			out = append(out, g.byte())
		}
		return out
	case 4, 5:
		n := int(g.byte() % 4)
		out := appendTestCBORArgument(nil, 0x80+(kind-4)*0x20, uint64(n), width)
		if kind == 5 {
			n *= 2
		}
		for range n {
			out = append(out, g.item(depth+1, false)...)
		}
		return out
	case 6:
		out := appendTestCBORArgument(nil, 0xc0, 4, width)
		return append(out, g.item(depth, true)...)
	case 8:
		out := []byte{0x84}
		for range cursor.Limbs {
			out = append(out, 0x1b)
			for range 8 {
				out = append(out, g.byte())
			}
		}
		return out
	default:
		var word uint64
		for range 8 {
			word = word<<8 | uint64(g.byte())
		}
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
		major := byte(0)
		if kind == 1 {
			major = 0x20
		} else if kind == 7 {
			major = 0xe0
			if width == 1 && word < 32 {
				word += 32
			}
		}
		return appendTestCBORArgument(nil, major, word, width)
	}
}
