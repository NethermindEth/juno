package fast_test

import (
	"strings"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/fast"
	"github.com/stretchr/testify/require"
)

// text encodes a short text string.
func text(s string) []byte { return append([]byte{0x60 | byte(len(s))}, s...) }

// pairs encodes a map from its pairs, keys and values already encoded, in the order given.
func pairs(items ...[]byte) []byte {
	out := []byte{0xa0 | byte(len(items)/2)}
	for _, item := range items {
		out = append(out, item...)
	}
	return out
}

type fieldValue struct {
	field int
	value uint64
}

// readStruct reads a map of integers with fields, skipping the values of unknown keys. The
// input may go on for trailer bytes after the map.
func readStruct(t *testing.T, data []byte, trailer int, fields *fast.Fields) ([]fieldValue, error) {
	t.Helper()
	r := fast.NewReader(data)
	s, err := r.OpenStruct(fields)
	if err != nil {
		return nil, err
	}
	var out []fieldValue
	for {
		field, err := r.NextField(&s)
		switch {
		case err != nil:
			return out, err
		case field == fast.Done:
			require.Equal(t, trailer, r.Len())
			return out, nil
		case field == fast.Unknown:
			require.NoError(t, r.Skip())
		default:
			value, err := r.Uint64()
			require.NoError(t, err)
			out = append(out, fieldValue{field, value})
		}
	}
}

func TestFieldsOrder(t *testing.T) {
	long := strings.Repeat("x", 30)
	fields := fast.NewFields("ccc", long, "bb", "a", "B0")
	require.Equal(t, []string{"a", "B0", "bb", "ccc", long}, fields.Order())
}

func TestNewFieldsRejects(t *testing.T) {
	require.Panics(t, func() { fast.NewFields("Fee", "fee") })
	require.Panics(t, func() { fast.NewFields("fé") })
	require.Panics(t, func() { fast.NewFields(strings.Repeat("x", 256)) })
}

func TestNextField(t *testing.T) {
	long := strings.Repeat("x", 30)
	mid := "abcdefghijklmnopqrst"
	fields := fast.NewFields("ccc", "a", "bb", long, mid)
	one, two, three := []byte{0x01}, []byte{0x02}, []byte{0x03}
	tests := map[string]struct {
		data []byte
		want []fieldValue
		err  error
	}{
		"canonical": {
			data: pairs(text("a"), one, text("bb"), two, text("ccc"), three),
			want: []fieldValue{{1, 1}, {2, 2}, {0, 3}},
		},
		"long key": {
			data: pairs(text("a"), one, []byte{0x78, 30}, []byte(long), two),
			want: []fieldValue{{1, 1}, {3, 2}},
		},
		"long key differing at the end": {
			data: pairs(text("a"), one, []byte{0x78, 30}, []byte(long[:29]+"y"), two),
			want: []fieldValue{{1, 1}},
		},
		"key of three words": {
			data: pairs(text(mid), one),
			want: []fieldValue{{4, 1}},
		},
		"key differing in its third word": {
			data: pairs(text(mid[:19]+"X"), one),
		},
		"key differing in its last byte": {
			data: pairs(text("ccd"), three),
		},
		"omitted field": {
			data: pairs(text("a"), one, text("ccc"), three),
			want: []fieldValue{{1, 1}, {0, 3}},
		},
		"out of order": {
			data: pairs(text("ccc"), three, text("bb"), two, text("a"), one),
			want: []fieldValue{{0, 3}, {2, 2}, {1, 1}},
		},
		"other case": {
			data: pairs(text("A"), one, text("cCc"), three),
			want: []fieldValue{{1, 1}, {0, 3}},
		},
		"unknown keys": {
			data: pairs(text("aa"), one, text("bb"), two, text("zzzz"), three),
			want: []fieldValue{{2, 2}},
		},
		"integer key": {
			data: pairs([]byte{0x01}, one, []byte{0x20}, two, text("a"), one),
			want: []fieldValue{{1, 1}},
		},
		"non-minimal key head": {
			data: pairs([]byte{0x78, 0x02, 'b', 'b'}, two),
			want: []fieldValue{{2, 2}},
		},
		"null":  {data: []byte{0xf6}},
		"empty": {data: pairs()},
		"repeated key": {
			data: pairs(text("a"), one, text("a"), two),
			want: []fieldValue{{1, 1}},
			err:  fast.ErrUnsupported,
		},
		"repeated key in another case": {
			data: pairs(text("a"), one, text("A"), two),
			want: []fieldValue{{1, 1}},
			err:  fast.ErrUnsupported,
		},
		"integer key overflows int64": {
			data: pairs([]byte{0x1b, 0x80, 0, 0, 0, 0, 0, 0, 0}, one),
			err:  fast.ErrType,
		},
		"byte string key": {
			data: pairs([]byte{0x41, 'a'}, one),
			err:  fast.ErrType,
		},
		"unknown key not UTF-8": {
			data: pairs([]byte{0x62, 0xff, 0xfe}, one),
			err:  fast.ErrSyntax,
		},
		"truncated key": {
			data: []byte{0xa1, 0x63, 'c', 'c'},
			err:  fast.ErrTruncated,
		},
		"not a map": {
			data: []byte{0x80},
			err:  fast.ErrType,
		},
	}
	for name, tt := range tests {
		// Keys are compared a word at a time when the input is long enough, and a byte at a
		// time near its end.
		for _, trailer := range []int{0, 32} {
			if trailer > 0 && tt.err == fast.ErrTruncated {
				continue // the trailer would complete it
			}
			t.Run(name, func(t *testing.T) {
				data := append(append([]byte{}, tt.data...), make([]byte, trailer)...)
				got, err := readStruct(t, data, trailer, fields)
				require.ErrorIs(t, err, tt.err)
				require.Equal(t, tt.want, got)
			})
		}
	}
}

// Done closes the map: a reader that reads two records, each a struct nested as deep as a
// record may, would otherwise fail the second with ErrDepth.
func TestNextFieldClosesTheMap(t *testing.T) {
	fields := fast.NewFields("a")
	var record []byte
	for range fast.MaxNestedLevels - 1 {
		record = append(record, 0x81)
	}
	record = append(record, pairs(text("a"), []byte{0x01})...)

	r := fast.NewReader(append(append([]byte{}, record...), record...))
	for range 2 {
		for range fast.MaxNestedLevels - 1 {
			_, err := r.ArrayLen()
			require.NoError(t, err)
		}
		got := []int{}
		s, err := r.OpenStruct(fields)
		require.NoError(t, err)
		for {
			field, err := r.NextField(&s)
			require.NoError(t, err)
			got = append(got, field)
			if field == fast.Done {
				break
			}
			require.NoError(t, r.Skip())
		}
		require.Equal(t, []int{0, fast.Done}, got)
		for range fast.MaxNestedLevels - 1 {
			r.Leave()
		}
	}
	require.Zero(t, r.Len())
}
