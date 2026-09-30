package cbor_test

import (
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
	"github.com/stretchr/testify/require"
)

func TestFxamackerGoldenBytes(t *testing.T) {
	golden := goldenBytes(t)

	for _, c := range goldenCases() {
		t.Run(c.name, func(t *testing.T) {
			b, err := fxamacker.Marshal(c.value)
			require.NoError(t, err)

			want, ok := golden[c.name]
			require.Truef(t, ok, "no vector for %q, the encoder wrote %s", c.name, hex.EncodeToString(b))
			require.Equal(t, want, hex.EncodeToString(b))

			stored, err := hex.DecodeString(want)
			require.NoError(t, err)

			back := reflect.New(reflect.TypeOf(c.value))
			require.NoError(t, fxamacker.Unmarshal(stored, back.Interface()))
			require.Equal(t, c.value, back.Elem().Interface())
		})
	}
	require.Equal(t, len(goldenCases()), len(golden), "a case lost its vector")
}

func TestFxamackerRawMessage(t *testing.T) {
	rawMessageContract[fxamacker.RawMessage](t, fxamacker.Marshal, fxamacker.Unmarshal)
}
