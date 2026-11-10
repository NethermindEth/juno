package cursor_test

import (
	"bytes"
	"fmt"
	"testing"
	"unicode/utf8"

	"github.com/NethermindEth/juno/utils/cbor/cursor"
	"github.com/stretchr/testify/require"
)

// Generate complete integer keys directly, so every fuzz case reaches the
// classification/range check instead of depending on byte mutations to get there.
func FuzzIntegerKey(f *testing.F) {
	for width := byte(0); width < 5; width++ {
		for _, value := range []uint64{0, 23, 24, 255, 256, 1<<32 - 1, 1<<63 - 1, 1 << 63, ^uint64(0)} {
			for _, negative := range []bool{false, true} {
				f.Add(value, width, negative, byte(32), byte(7), []byte{0xf6, 0xff})
			}
		}
	}
	f.Fuzz(func(t *testing.T, argument uint64, width byte, negative bool, depth, cut byte, suffix []byte) {
		width %= 5
		switch width {
		case 0:
			argument %= 24
		case 1:
			argument &= 1<<8 - 1
		case 2:
			argument &= 1<<16 - 1
		case 3:
			argument &= 1<<32 - 1
		}
		depth %= 33
		major := byte(0)
		if negative {
			major = 0x20
		}
		input := append(bytes.Repeat([]byte{0x81}, int(depth)), 0, 1, 2)
		start := len(input)
		input = appendTestCBORArgument(input, major, argument, width)
		end := len(input)
		input = append(input, suffix...)
		before := bytes.Clone(input)
		r := openSkipCursor(t, input, int(depth))
		key, text, err := r.Key()
		require.Nil(t, key)
		require.False(t, text)
		if argument < 1<<63 {
			require.NoError(t, err)
			require.Equal(t, end, r.Pos())
			require.True(t, bytes.Equal(suffix, r.Remaining()))
		} else {
			checkPayloadError(t, &r, err, cursor.ErrType, start, input[start:])
			key, text, err = r.Key()
			require.Nil(t, key)
			require.False(t, text)
			checkPayloadError(t, &r, err, cursor.ErrType, start, input[start:])
		}
		require.Equal(t, before, input)

		// All widths also exercise incomplete headers, including no first byte.
		short := input[:start+int(cut)%(end-start)]
		truncated := openSkipCursor(t, short, int(depth))
		for range 2 {
			key, text, err = truncated.Key()
			require.Nil(t, key)
			require.False(t, text)
			checkPayloadError(t, &truncated, err, cursor.ErrTruncated, start, short[start:])
		}
		require.Equal(t, before, input)
	})
}

func TestKeyWrongTypes(t *testing.T) {
	for _, major := range []byte{0x40, 0x80, 0xa0, 0xc0, 0xe0} {
		for width := byte(0); width < 5; width++ {
			t.Run(fmt.Sprintf("major%x/width%d", major, width), func(t *testing.T) {
				input := appendTestCBORArgument([]byte{0, 1, 2}, major, 0, width)
				input = append(input, 0xf6, 0xff)
				before := bytes.Clone(input)
				r := openSkipCursor(t, input, 0)
				kind := cursor.ErrType
				if major == 0xc0 {
					kind = cursor.ErrUnsupported
				}
				for range 2 {
					key, text, err := r.Key()
					require.Nil(t, key)
					require.False(t, text)
					checkPayloadError(t, &r, err, kind, 3, input[3:])
				}
				require.Equal(t, before, input)
			})
		}
	}
}

func TestKeyTextTruncation(t *testing.T) {
	for width := byte(0); width < 5; width++ {
		for _, length := range []uint64{1, 23, 24, 25, 255, 256, 1 << 63, ^uint64(0)} {
			if (width == 0 && length > 23) || (width == 1 && length > 255) ||
				(width == 2 && length > 65535) || (width == 3 && length > 1<<32-1) {
				continue
			}
			t.Run(fmt.Sprintf("width%d/length%d", width, length), func(t *testing.T) {
				for _, fill := range []byte{'a', 0xff} {
					// Even an invalid UTF-8 prefix must report truncation first.
					for cut := uint64(0); cut < min(length, 257); cut++ {
						input := appendTestCBORArgument([]byte{0, 1, 2}, 0x60, length, width)
						input = append(input, bytes.Repeat([]byte{fill}, int(cut))...)
						before := bytes.Clone(input)
						r := openSkipCursor(t, input, 0)
						for range 2 {
							key, text, err := r.Key()
							require.Nil(t, key)
							require.False(t, text)
							checkPayloadError(t, &r, err, cursor.ErrTruncated, 3, input[3:])
						}
						require.Equal(t, before, input)
					}
				}
			})
		}
	}
}

func FuzzTextKey(f *testing.F) {
	for width := byte(0); width < 5; width++ {
		for _, body := range [][]byte{nil, []byte("Number"), []byte("α世界"), {0xff}, {0xc2, 0x80}, {0xed, 0xa0, 0x80}} {
			for _, n := range []int{0, 7, 8, 9, 23, 24, 25, 255, 256} {
				f.Add(append(bytes.Repeat([]byte{'a'}, n), body...), width, byte(32), []byte{0xff, 0x00})
			}
		}
	}
	f.Fuzz(func(t *testing.T, body []byte, width, depth byte, suffix []byte) {
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
		input = appendTestCBORArgument(input, 0x60, uint64(len(body)), width)
		payloadStart := len(input)
		input = append(input, body...)
		end := len(input)
		input = append(input, suffix...)
		before := bytes.Clone(input)
		r := openSkipCursor(t, input, int(depth))
		key, text, err := r.Key()
		if utf8.Valid(body) {
			require.NoError(t, err)
			require.True(t, text)
			require.NotNil(t, key)
			require.True(t, bytes.Equal(body, key))
			require.Equal(t, end, r.Pos())
			require.True(t, bytes.Equal(suffix, r.Remaining()))
			require.Equal(t, before, input)
			if len(key) != 0 {
				key[0] ^= 0xff
				require.Equal(t, key[0], input[payloadStart], "Key must borrow the payload")
				key[0] ^= 0xff
			}
		} else {
			require.Nil(t, key)
			require.False(t, text)
			checkPayloadError(t, &r, err, cursor.ErrSyntax, start, input[start:])
			key, text, err = r.Key()
			require.Nil(t, key)
			require.False(t, text)
			checkPayloadError(t, &r, err, cursor.ErrSyntax, start, input[start:])
		}
		require.Equal(t, before, input)
	})
}

// One non-ASCII byte among ASCII bytes cannot form valid UTF-8. Test every
// possible byte in every position around word and short-key dispatch boundaries.
func TestKeyASCIIPositions(t *testing.T) {
	for _, length := range []int{1, 7, 8, 9, 23, 24, 25, 255} {
		for width := byte(0); width < 2; width++ {
			if width == 0 && length > 23 {
				continue
			}
			t.Run(fmt.Sprintf("length%d/width%d", length, width), func(t *testing.T) {
				body := bytes.Repeat([]byte{'a'}, length)
				input := appendTestCBORArgument([]byte{0, 1, 2}, 0x60, uint64(length), width)
				payloadStart := len(input)
				input = append(input, body...)
				end := len(input)
				input = append(input, 0xff, 0x00)
				for pos := range body {
					for value := range 256 {
						input[payloadStart+pos] = byte(value)
						r := openSkipCursor(t, input, 0)
						key, text, err := r.Key()
						if value < 128 {
							require.NoError(t, err)
							require.True(t, text)
							require.Equal(t, input[payloadStart:end], key)
							require.Equal(t, end, r.Pos())
							require.Equal(t, []byte{0xff, 0x00}, r.Remaining())
						} else {
							require.Nil(t, key)
							require.False(t, text)
							checkPayloadError(t, &r, err, cursor.ErrSyntax, 3, input[3:])
						}
						require.Equal(t, byte(value), input[payloadStart+pos])
					}
					input[payloadStart+pos] = 'a'
				}
			})
		}
	}
}
