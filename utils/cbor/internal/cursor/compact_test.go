package cursor_test

import (
	"bytes"
	"encoding/binary"
	"math/rand/v2"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/internal/cursor"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
	"github.com/stretchr/testify/require"
)

// montgomery returns the limbs of the felt whose canonical big-endian value is value.
func montgomery(t *testing.T, value []byte) [cursor.Limbs]uint64 {
	t.Helper()
	var padded [32]byte
	copy(padded[32-len(value):], value)
	element, err := fp.BigEndian.Element(&padded)
	require.NoError(t, err)
	return element
}

// trimmed returns n significant bytes, the first of them not zero.
func trimmed(n int) []byte {
	value := make([]byte, n)
	for i := range value {
		value[i] = byte(0x81 + i)
	}
	return value
}

func byteStringOf(body []byte) []byte {
	if len(body) < 24 {
		return append([]byte{0x40 | byte(len(body))}, body...)
	}
	return append([]byte{0x58, byte(len(body))}, body...)
}

func feltLimbs(a, b, c, d uint64) []byte {
	out := make([]byte, 32)
	binary.BigEndian.PutUint64(out[0:], d)
	binary.BigEndian.PutUint64(out[8:], c)
	binary.BigEndian.PutUint64(out[16:], b)
	binary.BigEndian.PutUint64(out[24:], a)
	return out
}

func TestCompactFelts(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want [cursor.Limbs]uint64
	}{
		{"zero", []byte{0x00}, montgomery(t, nil)},
		{"one", []byte{0x01}, montgomery(t, []byte{1})},
		{"largest uint form", []byte{0x17}, montgomery(t, []byte{23})},
		{"smallest byte form", []byte{0x41, 0x18}, montgomery(t, []byte{24})},
		{"one byte", []byte{0x41, 0xff}, montgomery(t, []byte{0xff})},
		{"two bytes", []byte{0x42, 0x01, 0x00}, montgomery(t, []byte{1, 0})},
		{"23 bytes", byteStringOf(trimmed(23)), montgomery(t, trimmed(23))},
		{"24 bytes", byteStringOf(trimmed(24)), montgomery(t, trimmed(24))},
		{"30 bytes", byteStringOf(trimmed(30)), montgomery(t, trimmed(30))},
		{
			"limbs",
			byteStringOf(feltLimbs(0, 0, 0, 0x0800000000000011)),
			[cursor.Limbs]uint64{0, 0, 0, 0x0800000000000011},
		},
		{"limbs of one", byteStringOf(feltLimbs(1, 0, 0, 0)), [cursor.Limbs]uint64{1, 0, 0, 0}},
		{
			"limbs with high bit",
			byteStringOf(feltLimbs(7, 8, 9, 0x0800000000000010)),
			[cursor.Limbs]uint64{7, 8, 9, 0x0800000000000010},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := cursor.NewCursor(append(append([]byte(nil), tt.in...), 0xf6))
			var got [cursor.Limbs]uint64
			require.NoError(t, c.Limbs(&got))
			require.Equal(t, tt.want, got)
			require.Equal(t, []byte{0xf6}, c.Remaining(), "it takes the felt and nothing after it")
		})
	}
}

func TestCompactFeltsRejects(t *testing.T) {
	maxed := ^uint64(0)
	tests := []struct {
		name string
		data []byte
		err  error
	}{
		{"an empty input", []byte{}, cursor.ErrTruncated},
		{"a uint head with a byte", []byte{0x18, 0x05}, cursor.ErrUnsupported},
		{"a wider uint", []byte{0x19, 0x01, 0x00}, cursor.ErrUnsupported},
		{"a byte string of one value below 24", []byte{0x41, 0x05}, cursor.ErrUnsupported},
		{"an empty byte string", []byte{0x40}, cursor.ErrUnsupported},
		{"a non-minimal length head", []byte{0x58, 0x02, 1, 2}, cursor.ErrUnsupported},
		{"a wider length head", []byte{0x59, 0x00, 0x02, 1, 2}, cursor.ErrUnsupported},
		{"an indefinite byte string", []byte{0x5f, 0x41, 1, 0xff}, cursor.ErrUnsupported},
		{"a leading zero", []byte{0x42, 0x00, 0x01}, cursor.ErrUnsupported},
		{"31 bytes", byteStringOf(trimmed(31)), cursor.ErrUnsupported},
		{"33 bytes", byteStringOf(trimmed(33)), cursor.ErrUnsupported},
		{
			"limbs at the modulus",
			byteStringOf(feltLimbs(1, 0, 0, 0x0800000000000011)),
			cursor.ErrUnsupported,
		},
		{
			"limbs above the modulus in the second limb",
			byteStringOf(feltLimbs(0, 1, 0, 0x0800000000000011)),
			cursor.ErrUnsupported,
		},
		{
			"limbs above the modulus in the third limb",
			byteStringOf(feltLimbs(0, 0, 1, 0x0800000000000011)),
			cursor.ErrUnsupported,
		},
		{
			"limbs above the modulus",
			byteStringOf(feltLimbs(maxed, maxed, maxed, maxed)),
			cursor.ErrUnsupported,
		},
		{"a truncated length head", []byte{0x58}, cursor.ErrTruncated},
		{"a truncated body", []byte{0x42, 0x01}, cursor.ErrTruncated},
		{"a negative integer", []byte{0x20}, cursor.ErrType},
		{"a text string", []byte{0x61, 'a'}, cursor.ErrType},
		{"a tag", []byte{0xc1, 0x01}, cursor.ErrUnsupported},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := cursor.NewCursor(tt.data)
			sentinel := [cursor.Limbs]uint64{9, 9, 9, 9}
			got := sentinel
			err := c.Limbs(&got)
			require.ErrorIs(t, err, tt.err)
			require.ErrorContains(t, err, "at byte 0", "the error says where")
			require.Equal(t, sentinel, got, "dst is written only on success")
			require.Zero(t, c.Pos(), "nothing is consumed on error")
		})
	}
}

func TestFeltSliceOfMixedForms(t *testing.T) {
	legacy := func(value []byte) []byte {
		out := []byte{0x84}
		for _, limb := range montgomery(t, value) {
			out = binary.BigEndian.AppendUint64(append(out, 0x1b), limb)
		}
		return out
	}

	t.Run("legacy and compact elements in one slice", func(t *testing.T) {
		in := append([]byte{0x83}, legacy([]byte{1})...)
		in = append(in, 0x02, 0x42, 0x01, 0x00)
		c := cursor.NewCursor(in)
		got, err := cursor.LimbSlice[[cursor.Limbs]uint64](&c)
		require.NoError(t, err)
		require.Equal(t, [][cursor.Limbs]uint64{
			montgomery(t, []byte{1}), montgomery(t, []byte{2}), montgomery(t, []byte{1, 0}),
		}, got)
		require.Zero(t, c.Len())
	})

	t.Run("a slice of many one-byte felts is not mistaken for a damaged count", func(t *testing.T) {
		in := append([]byte{0x98, 100}, make([]byte, 100)...)
		c := cursor.NewCursor(in)
		got, err := cursor.LimbSlice[[cursor.Limbs]uint64](&c)
		require.NoError(t, err)
		require.Len(t, got, 100)
	})

	t.Run("a count the input cannot hold is rejected", func(t *testing.T) {
		c := cursor.NewCursor([]byte{0x98, 100, 0x00, 0x00})
		_, err := cursor.LimbSlice[[cursor.Limbs]uint64](&c)
		require.ErrorIs(t, err, cursor.ErrUnsupported)
		require.Zero(t, c.Pos())
	})
}

// A trimmed value of every length reads as field arithmetic gives, whatever its bytes. The limb
// loads cross word boundaries at lengths 8, 16 and 24.
func TestCompactTrimmedLengths(t *testing.T) {
	random := rand.New(rand.NewPCG(1, 2))
	for n := 1; n <= 30; n++ {
		for range 300 {
			value := make([]byte, n)
			for i := range value {
				value[i] = byte(random.UintN(256))
			}
			// The first byte is not zero, and a single byte is 24 or more, as the form requires.
			value[0] = byte(24 + random.UintN(232))
			data := byteStringOf(value)
			c := cursor.NewCursor(data)
			var got [cursor.Limbs]uint64
			require.NoError(t, c.Limbs(&got))
			require.Equal(t, montgomery(t, value), got, "% x", value)
			require.Zero(t, c.Len())
		}
	}
}

func TestCompactFeltAtDepthLimit(t *testing.T) {
	legacy := []byte{0x84, 0, 0, 0, 0}
	compact := []byte{0x01}
	tests := []struct {
		name string
		// slice reads the felt with LimbSlice, which is a level of its own, so one fewer wrapper.
		slice   bool
		felt    []byte
		wantErr bool
	}{
		{"a legacy felt", false, legacy, true},
		{"a compact felt", false, compact, false},
		{"a slice of a legacy felt", true, legacy, true},
		{"a slice of a compact felt", true, compact, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := cursor.NewCursor(append(bytes.Repeat([]byte{0x81}, 32), tt.felt...))
			levels := 32
			if tt.slice {
				levels--
			}
			for range levels {
				count, err := c.ArrayLen()
				require.NoError(t, err)
				require.Equal(t, 1, count)
			}
			start := c.Pos()
			var err error
			if tt.slice {
				_, err = cursor.LimbSlice[[cursor.Limbs]uint64](&c)
			} else {
				var dst [cursor.Limbs]uint64
				err = c.Limbs(&dst)
			}
			if tt.wantErr {
				require.ErrorIs(t, err, cursor.ErrDepth)
				require.Equal(t, start, c.Pos())
				return
			}
			require.NoError(t, err)
			require.Zero(t, c.Len())
		})
	}
}
