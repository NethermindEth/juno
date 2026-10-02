package cbor_test

import (
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

// goldenEncoding checks exact stored bytes without relying on a matching decoder.
func goldenEncoding(
	t *testing.T,
	marshal func(any) ([]byte, error),
	cases []goldenCase,
) {
	t.Helper()
	golden := goldenBytes(t)
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

// goldenDecoding reads fixed bytes without calling an encoder.
func goldenDecoding(
	t *testing.T,
	unmarshal func([]byte, any) error,
	cases []goldenCase,
) {
	t.Helper()
	golden := goldenBytes(t)
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
