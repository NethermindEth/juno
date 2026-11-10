package cursor_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/cursor"
	"github.com/stretchr/testify/require"
)

// TestNextWanted reads the map of a struct with two fields wanted among others. The pairs of every
// other key are skipped, the first of two keys naming a field is the one read, and the pairs after
// the last field read are still checked.
func TestNextWanted(t *testing.T) {
	fields := cursor.NewFields("A", "Bee")
	// The keys are in canonical order, shorter first and then bytewise.
	canonical := slices.Concat(
		[]byte{0xa5},
		[]byte{0x61, 'A', 0x01},                                 // A: 1
		[]byte{0x62, 'A', 'a', 0x61, 'x'},                       // Aa: "x"
		[]byte{0x63, 'B', 'e', 'e', 0x02},                       // Bee: 2
		[]byte{0x64, 'L', 'a', 's', 't', 0xa1, 0x61, 'A', 0x03}, // Last: {"A": 3}
		[]byte{0x65, 'O', 't', 'h', 'e', 'r', 0x82, 0x01, 0x02}, // Other: [1, 2]
	)

	// Keys in other case, an integer key and repeats, of which the first is read.
	shuffled := slices.Concat(
		[]byte{0xa6},
		[]byte{0x01, 0x09},                // 1: 9
		[]byte{0x61, 'a', 0x01},           // "a": 1
		[]byte{0x63, 'b', 'E', 'E', 0x02}, // "bEE": 2
		[]byte{0x61, 'A', 0x07},           // "A": 7
		[]byte{0x63, 'B', 'e', 'e', 0x08}, // "Bee": 8
		[]byte{0x63, 'Z', 'e', 'd', 0x09}, // "Zed": 9
	)

	for name, data := range map[string][]byte{"canonical": canonical, "shuffled": shuffled} {
		t.Run(name, func(t *testing.T) {
			c := cursor.NewCursor(data)
			s, err := c.OpenStruct(fields)
			require.NoError(t, err)
			var got []uint64
			for {
				field, err := c.NextWanted(&s)
				require.NoError(t, err)
				if field == cursor.Done {
					break
				}
				value, err := c.Uint64()
				require.NoError(t, err)
				got = append(got, value)
			}
			require.Equal(t, []uint64{1, 2}, got)
			require.Zero(t, c.Len(), "the map is read to its end")
		})
	}

	// A pair after the last field read that is not well formed is an error.
	for name, data := range map[string][]byte{
		"invalid head":  {0xa2, 0x61, 'A', 0x01, 0x61, 'x', 0x1c},
		"byte key":      {0xa2, 0x61, 'A', 0x01, 0x41, 'x', 0x01},
		"not UTF-8 key": {0xa2, 0x61, 'A', 0x01, 0x62, 0xff, 0xfe, 0x01},
	} {
		t.Run(name, func(t *testing.T) {
			c := cursor.NewCursor(data)
			s, err := c.OpenStruct(fields)
			require.NoError(t, err)
			field, err := c.NextWanted(&s)
			require.NoError(t, err)
			require.Zero(t, field)
			_, err = c.Uint64()
			require.NoError(t, err)
			_, err = c.NextWanted(&s)
			require.Error(t, err)
		})
	}

	// A null struct has no pairs.
	c := cursor.NewCursor([]byte{0xf6})
	s, err := c.OpenStruct(fields)
	require.NoError(t, err)
	field, err := c.NextWanted(&s)
	require.NoError(t, err)
	require.Equal(t, cursor.Done, field)
}

// Use a non-preferred key width so lookup must reach the name matcher rather
// than matching the cached canonical encoding. Check every ASCII byte pair.
func TestFieldMatchASCII(t *testing.T) {
	for nameByte := byte(0); nameByte < 128; nameByte++ {
		fields := cursor.NewFields(string([]byte{nameByte}))
		for keyByte := byte(0); keyByte < 128; keyByte++ {
			input := []byte{0xa1, 0x79, 0, 1, keyByte, 0}
			c := cursor.NewCursor(input)
			s, err := c.OpenStruct(fields)
			require.NoError(t, err)
			field, err := c.NextField(&s)
			require.NoError(t, err)
			want := cursor.Unknown
			if strings.EqualFold(string([]byte{keyByte}), string([]byte{nameByte})) {
				want = 0
			}
			require.Equal(t, want, field, "key %x, name %x", keyByte, nameByte)
			require.Equal(t, 5, c.Pos())
			require.Equal(t, []byte{0}, c.Remaining())
			require.NoError(t, c.Skip())
			field, err = c.NextField(&s)
			require.NoError(t, err)
			require.Equal(t, cursor.Done, field)
		}
	}
}

func TestNextFieldTracksAllBits(t *testing.T) {
	names := make([]string, 64)
	for i := range names {
		names[i] = fmt.Sprintf("Field%02d", i)
	}
	fields := cursor.NewFields(names...)
	for width := byte(0); width < 5; width++ {
		for _, duplicate := range []int{0, 31, 32, 63} {
			t.Run(fmt.Sprintf("width%d/duplicate%d", width, duplicate), func(t *testing.T) {
				input := appendTestCBORArgument(nil, 0xa0, 65, 1)
				// A reverse order also exercises misses in the canonical lookup.
				for i := len(names) - 1; i >= 0; i-- {
					key := strings.ToUpper(names[i])
					input = appendTestCBORArgument(input, 0x60, uint64(len(key)), width)
					input = append(input, key...)
					input = appendTestCBORArgument(input, 0, uint64(i), 1)
				}
				key := strings.ToLower(names[duplicate])
				input = appendTestCBORArgument(input, 0x60, uint64(len(key)), width)
				input = append(input, key...)
				errorOffset := len(input)
				input = append(input, 0xf6, 0xff)
				before := slices.Clone(input)
				c := cursor.NewCursor(input)
				s, err := c.OpenStruct(fields)
				require.NoError(t, err)
				for i := len(names) - 1; i >= 0; i-- {
					field, err := c.NextField(&s)
					require.NoError(t, err)
					require.Equal(t, i, field)
					value, err := c.Uint64()
					require.NoError(t, err)
					require.Equal(t, uint64(i), value)
				}
				field, err := c.NextField(&s)
				require.Equal(t, cursor.Done, field)
				require.ErrorIs(t, err, cursor.ErrUnsupported)
				var detail *cursor.DecodeError
				require.ErrorAs(t, err, &detail)
				require.Equal(t, errorOffset, detail.Offset)
				require.Equal(t, errorOffset, c.Pos())
				require.Equal(t, []byte{0xf6, 0xff}, c.Remaining())
				require.Equal(t, before, input)
			})
		}
	}
}
