package fast_test

import (
	"bytes"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/fast"
	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
	"github.com/stretchr/testify/require"
)

// fxSkip is what fxamacker does with an item the destination has no field for: the item sits
// under an unknown key, so it is checked for well-formedness and skipped, never decoded.
// It returns the bytes after the item. The key's map adds one nesting level, which wrapped
// compensates for.
func fxSkip(item []byte) ([]byte, error) {
	var none struct{}
	return fxamacker.UnmarshalFirst(wrapped(item), &none)
}

// wrapped puts item under the one-character key "z" of a map.
func wrapped(item []byte) []byte {
	return append([]byte{0xa1, 0x61, 'z'}, item...)
}

func nested(levels int, leaf byte) []byte {
	return append(bytes.Repeat([]byte{0x81}, levels), leaf)
}

func TestSkipConsumesOneItem(t *testing.T) {
	type record struct {
		Name  string
		Items []uint64
		Blob  []byte
		Inner map[string][]int64
	}
	item := mustMarshal(t, record{
		Name:  "héllo",
		Items: []uint64{1, 300, 70000, 1 << 40},
		Blob:  bytes.Repeat([]byte{7}, 300),
		Inner: map[string][]int64{"a": {-1, 2}, "b": nil},
	})
	trailing := []byte{0xde, 0xad}

	r := fast.NewReader(append(append([]byte{}, item...), trailing...))
	require.NoError(t, r.Skip())
	require.Equal(t, len(item), r.Pos())
	require.Equal(t, trailing, r.Remaining())
}

func TestSkipErrors(t *testing.T) {
	tests := map[string]struct {
		data []byte
		err  error
	}{
		"empty":                    {nil, fast.ErrTruncated},
		"truncated string":         {[]byte{0x63, 'a'}, fast.ErrTruncated},
		"truncated array":          {[]byte{0x82, 0x01}, fast.ErrTruncated},
		"truncated map value":      {[]byte{0xa1, 0x01}, fast.ErrTruncated},
		"count beyond input":       {[]byte{0x9b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, fast.ErrTruncated},
		"reserved info":            {[]byte{0x1d}, fast.ErrSyntax},
		"long simple below 32":     {[]byte{0xf8, 0x1f}, fast.ErrSyntax},
		"stray break":              {[]byte{0xff}, fast.ErrSyntax},
		"indefinite array":         {[]byte{0x9f, 0x01, 0xff}, fast.ErrUnsupported},
		"indefinite inside":        {[]byte{0x81, 0x7f, 0xff}, fast.ErrUnsupported},
		"tag without content":      {[]byte{0xc2}, fast.ErrTruncated},
		"nesting past the limit":   {nested(fast.MaxNestedLevels, 0x80), fast.ErrDepth},
		"deep tags past the limit": {bytes.Repeat([]byte{0xc6}, 1<<16), fast.ErrDepth},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			r := fast.NewReader(tt.data)
			require.ErrorIs(t, r.Skip(), tt.err)
			require.Zero(t, r.Pos(), "a failed skip must not consume")
		})
	}
}

func TestSkipLongSimpleValues(t *testing.T) {
	for _, data := range [][]byte{
		{0xf8, 0x20},                         // simple(32)
		{0xf9, 0x3c, 0x00},                   // half float 1.0
		{0xfa, 0x3f, 0x80, 0, 0},             // float32 1.0
		{0xfb, 0x3f, 0xf0, 0, 0, 0, 0, 0, 0}, // float64 1.0
	} {
		r := fast.NewReader(data)
		require.NoError(t, r.Skip())
		require.Zero(t, r.Len())
	}
}

// The nesting limit must agree with the decoder it replaces at the boundary.
func TestSkipNestingMatchesFxamacker(t *testing.T) {
	for levels := range fast.MaxNestedLevels + 3 {
		for _, leaf := range []byte{0x01, 0x80} {
			data := nested(levels, leaf)
			_, fxErr := fxSkip(data)

			r := fast.NewReader(wrapped(data))
			err := r.Skip()
			require.Equalf(t, fxErr == nil, err == nil, "levels=%d leaf=%#x fx=%v fast=%v", levels, leaf, fxErr, err)
		}
	}
}

func TestSkipTagChainsMatchFxamacker(t *testing.T) {
	for chain := range fast.MaxNestedLevels + 3 {
		for _, content := range [][]byte{{0x01}, {0x80}, {0x81, 0x01}} {
			data := append(bytes.Repeat([]byte{0xc6}, chain), content...)
			_, fxErr := fxSkip(data)

			r := fast.NewReader(wrapped(data))
			err := r.Skip()
			require.Equalf(t, fxErr == nil, err == nil, "chain=%d content=%x fx=%v fast=%v", chain, content, fxErr, err)
		}
	}
}

// Text is validated where it is returned, not where it is skipped, as in fxamacker.
func TestSkipIgnoresInvalidUTF8(t *testing.T) {
	r := fast.NewReader([]byte{0x81, 0x62, 0xc3, 0x28})
	require.NoError(t, r.Skip())
	require.Zero(t, r.Len())
}

func TestSkipAcceptsWhatFxamackerSkips(t *testing.T) {
	for _, data := range [][]byte{
		{0xc2, 0x01},       // tag 2 over an integer
		{0xc0, 0x01},       // tag 0 over an integer
		{0xc1, 0x80},       // tag 1 over an array
		{0x62, 0xc3, 0x28}, // invalid UTF-8
	} {
		_, fxErr := fxSkip(data)
		require.NoError(t, fxErr)

		r := fast.NewReader(data)
		require.NoError(t, r.Skip())
		require.Zero(t, r.Len())
	}
}
