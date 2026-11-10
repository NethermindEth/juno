package cursor_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/cursor"
	"github.com/stretchr/testify/require"
)

func TestKeyHeadErrors(t *testing.T) {
	for h := range 256 {
		head := byte(h)
		major, info := head>>5, head&31
		switch {
		case info >= 28:
			kind := cursor.ErrSyntax
			reason := "invalid additional information"
			if info == 31 && major >= 2 && major <= 5 {
				kind, reason = cursor.ErrUnsupported, "indefinite-length item"
			} else if head == 0xff {
				reason = "break code outside an indefinite-length item"
			}
			// Cross the word-cache boundary without changing the invalid head.
			for _, tail := range []int{0, 22, 23, 24, 32} {
				t.Run(fmt.Sprintf("%02x/tail%d", head, tail), func(t *testing.T) {
					wire := append([]byte{head}, bytes.Repeat([]byte{0xff}, tail)...)
					checkKeyHeadFailure(t, wire, kind, reason)
				})
			}
		case info >= 24:
			width := 1 << (info - 24)
			for available := range width {
				t.Run(fmt.Sprintf("%02x/argument%d", head, available), func(t *testing.T) {
					wire := append([]byte{head}, bytes.Repeat([]byte{0xff}, available)...)
					checkKeyHeadFailure(t, wire, cursor.ErrTruncated, "input ends inside the head")
				})
			}
		}
	}
}

// Exercise the public readers after a nonzero prefix, including a fresh field
// search, a previously seen field, and an exhausted ordered field table.
func checkKeyHeadFailure(t *testing.T, wire []byte, kind error, reason string) {
	t.Helper()
	input := append([]byte{0, 1, 2}, wire...)
	before := bytes.Clone(input)
	r := openSkipCursor(t, input, 0)
	for range 2 {
		key, text, err := r.Key()
		require.Nil(t, key)
		require.False(t, text)
		checkPayloadError(t, &r, err, kind, 3, wire)
		require.ErrorContains(t, err, reason)
	}
	require.Equal(t, before, input)

	fields := cursor.NewFields("a", "z")
	for _, prior := range []int{-1, 0, 1} {
		input = []byte{0, 1, 2, 0xa1}
		if prior >= 0 {
			input[3] = 0xa2
			input = append(input, 0x61, []byte{'a', 'z'}[prior], 0x00)
		}
		start := len(input)
		input = append(input, wire...)
		before = bytes.Clone(input)
		r = openSkipCursor(t, input, 0)
		s, err := r.OpenStruct(fields)
		if prior < 0 && len(wire) < 2 {
			// A one-byte remainder cannot hold even a minimal key/value pair,
			// so OpenStruct rejects the count before looking at the key.
			checkPayloadError(t, &r, err, cursor.ErrTruncated, 3, input[3:])
			require.Equal(t, before, input)
			continue
		}
		require.NoError(t, err)
		if prior >= 0 {
			field, err := r.NextField(&s)
			require.NoError(t, err)
			require.Equal(t, prior, field)
			value, err := r.Uint64()
			require.NoError(t, err)
			require.Zero(t, value)
		}
		field, err := r.NextField(&s)
		require.Equal(t, cursor.Done, field)
		checkPayloadError(t, &r, err, kind, start, wire)
		require.ErrorContains(t, err, reason)
		// The failed key consumed the last pair slot but no input bytes.
		field, err = r.NextField(&s)
		require.NoError(t, err)
		require.Equal(t, cursor.Done, field)
		require.Equal(t, start, r.Pos())
		require.Equal(t, wire, r.Remaining())
		require.Equal(t, before, input)
	}
}
