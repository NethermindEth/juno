package cbor_test

import (
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

// assertEncodesToGolden checks exact stored bytes without relying on a matching decoder.
func assertEncodesToGolden(
	t *testing.T,
	marshal func(any) ([]byte, error),
	golden map[string]string,
	cases []goldenCase,
) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			encoded, err := marshal(c.value)
			require.NoError(t, err)

			want, ok := golden[c.name]
			require.Truef(t, ok, "no vector for %q, the encoder wrote %s",
				c.name, hex.EncodeToString(encoded))
			require.Equal(t, want, hex.EncodeToString(encoded))
		})
	}
}

// assertDecodesFromGolden checks decoding fixed bytes without calling an encoder.
func assertDecodesFromGolden(
	t *testing.T,
	unmarshal func([]byte, any) error,
	golden map[string]string,
	cases []goldenCase,
) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			encoded, ok := golden[c.name]
			require.Truef(t, ok, "no vector for %q", c.name)
			stored, err := hex.DecodeString(encoded)
			require.NoError(t, err)

			back := reflect.New(reflect.TypeOf(c.value))
			require.NoError(t, unmarshal(stored, back.Interface()))
			require.Equal(t, c.value, back.Elem().Interface())
		})
	}
}

// legacyCases keeps the cases that have a vector in golden, failing on a vector with no case.
func legacyCases(t *testing.T, golden map[string]string, cases []goldenCase) []goldenCase {
	t.Helper()
	var matched []goldenCase
	for _, c := range cases {
		if _, ok := golden[c.name]; ok {
			matched = append(matched, c)
		}
	}
	require.Len(t, matched, len(golden), "a legacy vector lost its case")
	return matched
}
