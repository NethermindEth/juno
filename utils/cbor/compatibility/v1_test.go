package cbor_test

import (
	"encoding/hex"
	"reflect"
	"testing"

	cborv1 "github.com/NethermindEth/juno/utils/cbor/v1"
	"github.com/stretchr/testify/require"
)

func TestV1GoldenBytes(t *testing.T) {
	golden := goldenBytes(t)

	for _, c := range goldenCases() {
		t.Run(c.name, func(t *testing.T) {
			b, err := cborv1.Marshal(c.value)
			require.NoError(t, err)

			want, ok := golden[c.name]
			require.Truef(t, ok, "no vector for %q, the encoder wrote %s", c.name, hex.EncodeToString(b))
			require.Equal(t, want, hex.EncodeToString(b))

			stored, err := hex.DecodeString(want)
			require.NoError(t, err)

			back := reflect.New(reflect.TypeOf(c.value))
			require.NoError(t, cborv1.Unmarshal(stored, back.Interface()))
			require.Equal(t, c.value, back.Elem().Interface())
		})
	}
	require.Equal(t, len(goldenCases()), len(golden), "a case lost its vector")
}

func TestV1RawMessage(t *testing.T) {
	rawMessageContract[cborv1.RawMessage](t, cborv1.Marshal, cborv1.Unmarshal)
}
