package cursor_test

import (
	"slices"
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
