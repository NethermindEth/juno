package cbor_test

import (
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/utils/cbor/ugorji"
	"github.com/stretchr/testify/require"
)

// Decode the golden records with Ugorji directly.
func TestUgorjiSupportedGoldenReads(t *testing.T) {
	cases := ugorjiGoldenCases()
	require.NotEmpty(t, cases)
	assertDecodesFromGolden(t, ugorji.Unmarshal, cases)
}

// Storage reads decode into **T from a buffer the database releases after the
// read, so decoded records must not reference it.
func TestUgorjiReadsDoNotRetainInput(t *testing.T) {
	golden := goldenBytes(t)
	for _, c := range ugorjiGoldenCases() {
		t.Run(c.name, func(t *testing.T) {
			input, err := hex.DecodeString(golden[c.name])
			require.NoError(t, err)

			expected := reflect.New(reflect.TypeOf(c.value))
			expected.Elem().Set(reflect.ValueOf(c.value))
			actual := reflect.New(expected.Type())
			require.NoError(t, ugorji.Unmarshal(input, actual.Interface()))
			clear(input)
			require.Equal(t, expected.Interface(), actual.Elem().Interface())
		})
	}
}

func ugorjiGoldenCases() []goldenCase {
	var cases []goldenCase
	for _, c := range goldenCases() {
		switch c.value.(type) {
		case core.Header, core.StateUpdate:
			cases = append(cases, c)
		}
	}
	return cases
}
