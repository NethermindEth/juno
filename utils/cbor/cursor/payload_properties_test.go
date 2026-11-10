package cursor_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/cursor"
	"github.com/stretchr/testify/require"
)

// Every generated input is a complete byte payload, including non-preferred
// length widths, so fuzzing exercises borrowing and copying rather than syntax.
func FuzzBytePayload(f *testing.F) {
	for width := byte(0); width < 5; width++ {
		for _, n := range []int{0, 1, 23, 24, 255, 256} {
			f.Add(bytes.Repeat([]byte{0xff}, n), width, byte(32), uint16(n/2), []byte{0xf6, 0xff})
		}
	}
	f.Fuzz(func(t *testing.T, body []byte, width, depth byte, cut uint16, suffix []byte) {
		if len(body) > 4096 {
			body = body[:4096]
		}
		width %= 5
		if width == 0 && len(body) > 23 {
			width = 1
		}
		if width == 1 && len(body) > 255 {
			width = 2
		}
		depth %= 33
		input := append(bytes.Repeat([]byte{0x81}, int(depth)), 0, 1, 2)
		start := len(input)
		input = appendTestCBORArgument(input, 0x40, uint64(len(body)), width)
		payloadStart := len(input)
		input = append(input, body...)
		end := len(input)
		input = append(input, suffix...)
		before := bytes.Clone(input)
		borrower := openSkipCursor(t, input, int(depth))
		borrowed, err := borrower.BytesBorrowed()
		require.NoError(t, err)
		require.NotNil(t, borrowed)
		require.True(t, bytes.Equal(body, borrowed))
		require.Equal(t, end, borrower.Pos())
		require.True(t, bytes.Equal(suffix, borrower.Remaining()))

		copier := openSkipCursor(t, input, int(depth))
		copied, err := copier.Bytes()
		require.NoError(t, err)
		require.NotNil(t, copied)
		require.True(t, bytes.Equal(body, copied))
		require.Equal(t, end, copier.Pos())
		require.True(t, bytes.Equal(suffix, copier.Remaining()))
		require.Equal(t, before, input)
		if len(body) != 0 {
			borrowed[0] ^= 0xff
			require.Equal(t, borrowed[0], input[payloadStart])
			require.Equal(t, body[0], copied[0])
			borrowed[0] ^= 0xff
			copied[0] ^= 0xff
			require.Equal(t, before, input)
		}

		// A complete length header followed by a short body must roll back.
		if len(body) != 0 {
			short := input[:payloadStart+int(cut)%len(body)]
			for _, copyOutput := range []bool{false, true} {
				r := openSkipCursor(t, short, int(depth))
				for range 2 {
					var got []byte
					if copyOutput {
						got, err = r.Bytes()
					} else {
						got, err = r.BytesBorrowed()
					}
					require.Nil(t, got)
					checkPayloadError(t, &r, err, cursor.ErrTruncated, start, short[start:])
				}
			}
		}
	})
}

func TestStringPayloadBoundaries(t *testing.T) {
	vectors := []struct {
		name  string
		body  []byte
		valid bool
	}{
		{"empty", nil, true},
		{"ascii", []byte{0, 0x7f}, true},
		{"scalarBoundaries", []byte("\u0080\u07ff\u0800\ud7ff\ue000\uffff\U00010000\U0010ffff"), true},
		{"continuation", []byte{0x80}, false},
		{"overlong2", []byte{0xc0, 0x80}, false},
		{"overlong3", []byte{0xe0, 0x9f, 0xbf}, false},
		{"surrogate", []byte{0xed, 0xa0, 0x80}, false},
		{"overlong4", []byte{0xf0, 0x8f, 0xbf, 0xbf}, false},
		{"aboveMaximum", []byte{0xf4, 0x90, 0x80, 0x80}, false},
		{"missingContinuation", []byte{0xf0, 0x90, 0x80}, false},
	}
	// Put each boundary after prefixes around every ASCII batching threshold.
	for _, prefix := range []int{0, 1, 8, 9, 16, 17, 24, 25, 32, 33, 56, 57, 64, 65, 128} {
		for _, vector := range vectors {
			for width := byte(0); width < 5; width++ {
				body := append(bytes.Repeat([]byte{'a'}, prefix), vector.body...)
				if width == 0 && len(body) > 23 {
					continue
				}
				t.Run(fmt.Sprintf("%s/prefix%d/width%d", vector.name, prefix, width), func(t *testing.T) {
					input := appendTestCBORArgument([]byte{0, 1, 2}, 0x60, uint64(len(body)), width)
					payloadStart := len(input)
					input = append(input, body...)
					end := len(input)
					input = append(input, 0xf6, 0xff)
					before := bytes.Clone(input)
					r := openSkipCursor(t, input, 0)
					got, err := r.String()
					if vector.valid {
						require.NoError(t, err)
						require.Equal(t, string(body), got)
						require.Equal(t, end, r.Pos())
						require.Equal(t, []byte{0xf6, 0xff}, r.Remaining())
						require.Equal(t, before, input)
						if len(body) != 0 {
							input[payloadStart] ^= 0xff
							require.Equal(t, string(body), got)
						}
					} else {
						require.Empty(t, got)
						checkPayloadError(t, &r, err, cursor.ErrSyntax, 3, input[3:])
						got, err = r.String()
						require.Empty(t, got)
						checkPayloadError(t, &r, err, cursor.ErrSyntax, 3, input[3:])
						require.Equal(t, before, input)
					}
					// Payload truncation precedes UTF-8 validation, even when the
					// available prefix already contains an invalid encoding.
					for cut := 0; cut < len(body); cut++ {
						short := before[:payloadStart+cut]
						truncated := openSkipCursor(t, short, 0)
						got, err = truncated.String()
						require.Empty(t, got)
						checkPayloadError(t, &truncated, err, cursor.ErrTruncated, 3, short[3:])
					}
				})
			}
		}
	}
}

func TestPayloadDeclaredLengthOverflow(t *testing.T) {
	for _, length := range []uint64{1 << 63, ^uint64(0)} {
		for _, major := range []byte{0x40, 0x60} {
			t.Run(fmt.Sprintf("major%x/length%d", major, length), func(t *testing.T) {
				input := appendTestCBORArgument([]byte{0, 1, 2}, major, length, 4)
				input = append(input, bytes.Repeat([]byte{'a'}, 64)...)
				before := bytes.Clone(input)
				r := openSkipCursor(t, input, 0)
				for range 2 {
					var err error
					if major == 0x40 {
						var got []byte
						got, err = r.BytesBorrowed()
						require.Nil(t, got)
					} else {
						var got string
						got, err = r.String()
						require.Empty(t, got)
					}
					checkPayloadError(t, &r, err, cursor.ErrTruncated, 3, input[3:])
				}
				require.Equal(t, before, input)
			})
		}
	}
}

func checkPayloadError(t *testing.T, r *cursor.Cursor, err, kind error, start int, unread []byte) {
	t.Helper()
	require.ErrorIs(t, err, kind)
	var detail *cursor.DecodeError
	require.ErrorAs(t, err, &detail)
	require.Equal(t, start, detail.Offset)
	require.Equal(t, start, r.Pos())
	require.True(t, bytes.Equal(unread, r.Remaining()))
}
