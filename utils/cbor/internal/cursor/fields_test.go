package cursor_test

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/internal/cursor"
	"github.com/stretchr/testify/require"
)

// TestNextWanted reads the map of a struct with two fields wanted among others. The pairs of every
// other key are skipped, the first of two keys naming a field is the one read, and the pairs after
// the last field read are still checked.
func TestNextWanted(t *testing.T) {
	fields, err := cursor.NewFields("A", "Bee")
	require.NoError(t, err)
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

func newFields(t *testing.T, names ...string) cursor.Fields {
	t.Helper()
	fields, err := cursor.NewFields(names...)
	require.NoError(t, err)
	return fields
}

// textItem encodes s, of fewer than 256 bytes, as a text string.
func textItem(s string) []byte {
	if len(s) < 24 {
		return append([]byte{0x60 | byte(len(s))}, s...)
	}
	return append([]byte{0x78, byte(len(s))}, s...)
}

func TestNewFields(t *testing.T) {
	names := func(n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = fmt.Sprintf("f%d", i)
		}
		return out
	}

	tests := []struct {
		name  string
		names []string
		ok    bool
	}{
		{"no names", nil, true},
		{"distinct names", []string{"A", "Bee"}, true},
		{"64 names", names(64), true},
		{"more than 64 names", names(65), false},
		{"a name of 255 bytes", []string{strings.Repeat("x", 255)}, true},
		{"a name of 256 bytes", []string{strings.Repeat("x", 256)}, false},
		{"a name that is not ASCII", []string{"café"}, false},
		{"a name repeated", []string{"A", "B", "A"}, false},
		{"a name repeated in another case", []string{"Amount", "amOUNT"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := cursor.NewFields(tt.names...)
			if tt.ok {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

// pair is a field NextField named and the integer value that followed it.
type pair struct{ field, value int }

// readPairs reads a struct of integer values to its end, which is followed by padding bytes. A key
// that names no field is Unknown, and its value is skipped and read as 0.
func readPairs(t *testing.T, data []byte, fields cursor.Fields, padding int) []pair {
	t.Helper()
	c := cursor.NewCursor(slices.Concat(data, bytes.Repeat([]byte{0xf6}, padding))) // nulls
	s, err := c.OpenStruct(fields)
	require.NoError(t, err)

	var got []pair
	for {
		field, err := c.NextField(&s)
		require.NoError(t, err)
		switch field {
		case cursor.Done:
			require.Equal(t, padding, c.Len(), "the map is read to its end")
			return got
		case cursor.Unknown:
			require.NoError(t, c.Skip())
			got = append(got, pair{field, 0})
		default:
			value, err := c.Uint64()
			require.NoError(t, err)
			got = append(got, pair{field, int(value)})
		}
	}
}

func TestNextField(t *testing.T) {
	fields := newFields(t, "A", "Bee")
	long := strings.Repeat("x", 255)

	tests := []struct {
		name   string
		fields cursor.Fields
		data   []byte
		want   []pair
	}{
		{
			"keys in order",
			fields,
			slices.Concat([]byte{0xa2}, textItem("A"), []byte{0x01}, textItem("Bee"), []byte{0x02}),
			[]pair{{0, 1}, {1, 2}},
		},
		{
			"keys out of order",
			fields,
			slices.Concat([]byte{0xa2}, textItem("Bee"), []byte{0x02}, textItem("A"), []byte{0x01}),
			[]pair{{1, 2}, {0, 1}},
		},
		{
			"keys in another case",
			fields,
			slices.Concat([]byte{0xa2}, textItem("a"), []byte{0x01}, textItem("bEE"), []byte{0x02}),
			[]pair{{0, 1}, {1, 2}},
		},
		{
			"a key that names no field",
			fields,
			slices.Concat(
				[]byte{0xa3},
				textItem("A"), []byte{0x01},
				textItem("Zed"), []byte{0x07},
				textItem("Bee"), []byte{0x02},
			),
			[]pair{{0, 1}, {cursor.Unknown, 0}, {1, 2}},
		},
		{
			"an integer key",
			fields,
			slices.Concat([]byte{0xa2, 0x01, 0x09}, textItem("A"), []byte{0x01}),
			[]pair{{cursor.Unknown, 0}, {0, 1}},
		},
		{
			"a name that is a prefix of another",
			newFields(t, "Id", "Identifier"),
			slices.Concat([]byte{0xa2}, textItem("Identifier"), []byte{0x02}, textItem("Id"), []byte{0x01}),
			[]pair{{1, 2}, {0, 1}},
		},
		{
			"a name of 255 bytes",
			newFields(t, "A", long),
			slices.Concat([]byte{0xa2}, textItem(long), []byte{0x02}, textItem("A"), []byte{0x01}),
			[]pair{{1, 2}, {0, 1}},
		},
		{"an empty map", fields, []byte{0xa0}, nil},
		{
			"a struct with no fields",
			cursor.Fields{},
			slices.Concat([]byte{0xa1}, textItem("A"), []byte{0x01}),
			[]pair{{cursor.Unknown, 0}},
		},
	}
	// NextField compares a key a word at a time when 24 bytes of input are left, and byte by byte
	// otherwise, so every case runs with and without that many bytes after the map.
	paddings := map[string]int{"at the end of the input": 0, "before 24 more bytes": 24}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for name, padding := range paddings {
				t.Run(name, func(t *testing.T) {
					require.Equal(t, tt.want, readPairs(t, tt.data, tt.fields, padding))
				})
			}
		})
	}

	t.Run("a field named twice is unsupported", func(t *testing.T) {
		for name, second := range map[string]string{"same case": "A", "other case": "a"} {
			t.Run(name, func(t *testing.T) {
				data := slices.Concat([]byte{0xa2}, textItem("A"), []byte{0x01}, textItem(second), []byte{0x02})
				c := cursor.NewCursor(data)
				s, err := c.OpenStruct(fields)
				require.NoError(t, err)

				field, err := c.NextField(&s)
				require.NoError(t, err)
				require.Zero(t, field)
				_, err = c.Uint64()
				require.NoError(t, err)

				_, err = c.NextField(&s)
				require.ErrorIs(t, err, cursor.ErrUnsupported)
			})
		}
	})

	t.Run("a key that is not valid is refused", func(t *testing.T) {
		c := cursor.NewCursor([]byte{0xa1, 0x41, 'A', 0x01}) // a byte string
		s, err := c.OpenStruct(fields)
		require.NoError(t, err)

		_, err = c.NextField(&s)
		require.ErrorIs(t, err, cursor.ErrType)
	})

	// Done closes the map, so the nesting level it took is free again.
	t.Run("Done closes the map", func(t *testing.T) {
		next := map[string]func(*cursor.Cursor, *cursor.Struct) (int, error){
			"NextField":  (*cursor.Cursor).NextField,
			"NextWanted": (*cursor.Cursor).NextWanted,
		}
		for name, next := range next {
			t.Run(name, func(t *testing.T) {
				data := slices.Concat([]byte{0xa1}, textItem("A"), []byte{0x01}, nested(deeperThanLimit, 0x01))
				c := cursor.NewCursor(data)
				s, err := c.OpenStruct(fields)
				require.NoError(t, err)

				field, err := next(&c, &s)
				require.NoError(t, err)
				require.Zero(t, field)
				_, err = c.Uint64()
				require.NoError(t, err)
				field, err = next(&c, &s)
				require.NoError(t, err)
				require.Equal(t, cursor.Done, field)
				require.Equal(t, nestingLimit(t), openable(c))

				field, err = next(&c, &s)
				require.NoError(t, err)
				require.Equal(t, cursor.Done, field, "Done is returned again")
				require.Equal(t, nestingLimit(t), openable(c), "and closes nothing more")
			})
		}
	})
}

func TestOpenStruct(t *testing.T) {
	fields := newFields(t, "A")

	t.Run("a map is opened", func(t *testing.T) {
		c := cursor.NewCursor(slices.Concat([]byte{0xa0}, nested(deeperThanLimit, 0x01)))

		_, err := c.OpenStruct(fields)
		require.NoError(t, err)
		require.Equal(t, 1, c.Pos())
		require.Equal(t, nestingLimit(t)-1, openable(c), "and takes a nesting level")
	})

	t.Run("null has no pairs", func(t *testing.T) {
		for _, head := range []byte{0xf6, 0xf7} {
			c := cursor.NewCursor([]byte{head})
			s, err := c.OpenStruct(fields)
			require.NoError(t, err)
			require.Equal(t, 1, c.Pos())

			field, err := c.NextField(&s)
			require.NoError(t, err)
			require.Equal(t, cursor.Done, field)
		}
	})
}

func TestKey(t *testing.T) {
	t.Run("a text key is returned from the input", func(t *testing.T) {
		for _, key := range []string{"", "A", "café", strings.Repeat("k", 23), strings.Repeat("k", 100)} {
			data := slices.Concat(textItem(key), []byte{0x01})
			c := cursor.NewCursor(data)

			got, isText, err := c.Key()
			require.NoError(t, err)
			require.True(t, isText)
			require.Equal(t, key, string(got))
			require.Equal(t, []byte{0x01}, c.Remaining())
			if key != "" {
				require.Same(t, &data[len(data)-1-len(key)], &got[0], "it aliases the input")
			}
		}
	})

	t.Run("an integer key is consumed and not text", func(t *testing.T) {
		for name, data := range map[string][]byte{
			"unsigned": {0x01},
			"negative": {0x20},
			"wide":     {0x19, 0x01, 0x00},
		} {
			t.Run(name, func(t *testing.T) {
				c := cursor.NewCursor(slices.Concat(data, []byte{0xf6}))

				key, isText, err := c.Key()
				require.NoError(t, err)
				require.False(t, isText)
				require.Nil(t, key)
				require.Equal(t, []byte{0xf6}, c.Remaining())
			})
		}
	})

	t.Run("a key that is refused consumes nothing", func(t *testing.T) {
		tests := []struct {
			name string
			data []byte
			err  error
		}{
			{"not UTF-8", []byte{0x62, 0xff, 0xfe}, cursor.ErrSyntax},
			{
				"not UTF-8 in a long key",
				slices.Concat([]byte{0x78, 30}, bytes.Repeat([]byte{0xff}, 30)),
				cursor.ErrSyntax,
			},
			{"a long key cut short", []byte{0x78, 30, 'A'}, cursor.ErrTruncated},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				c := cursor.NewCursor(tt.data)

				_, _, err := c.Key()
				require.ErrorIs(t, err, tt.err)
				require.Zero(t, c.Pos())
			})
		}
	})
}
