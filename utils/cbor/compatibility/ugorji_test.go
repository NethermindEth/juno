package cbor_test

import (
	"bytes"
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
		case core.Header, core.StateUpdate, core.TransactionReceipt:
			cases = append(cases, c)
		}
	}
	return cases
}

func cborText(s string) []byte { return append([]byte{0x60 | byte(len(s))}, s...) }

// legacyHeader returns the stored populated header with the gas price keys
// older databases hold: "GasPrice" and "GasPriceSTRK" before #2335. A non-nil
// entry is appended to the map.
func legacyHeader(t *testing.T, entry []byte) []byte {
	t.Helper()
	data, err := hex.DecodeString(goldenBytes(t)["Header, populated"])
	require.NoError(t, err)

	for _, rename := range [][2]string{{"gasprice", "GasPrice"}, {"gaspricestrk", "GasPriceSTRK"}} {
		current, legacy := cborText(rename[0]), cborText(rename[1])
		require.Equal(t, 1, bytes.Count(data, current), rename[0])
		data = bytes.Replace(data, current, legacy, 1)
	}
	if entry != nil {
		// The field count fits in the map's initial byte; append one more entry.
		require.Equal(t, byte(0xa0), data[0]&0xe0)
		require.Less(t, data[0]&0x1f, byte(23))
		data[0]++
		data = append(data, entry...)
	}
	return data
}

// Ugorji reads legacy header keys, including "ExtraData" from before #1498,
// without losing the L1 gas prices, and rejects unknown keys and legacy values
// that are not felts.
func TestUgorjiReadsLegacyHeaderKeys(t *testing.T) {
	for _, tc := range []struct {
		name    string
		entry   []byte
		wantErr string
	}{
		{name: "gas prices"},
		{name: "ExtraData null", entry: append(cborText("ExtraData"), 0xf6)},
		{name: "ExtraData felt", entry: append(cborText("ExtraData"), 0x84, 1, 2, 3, 4)},
		{name: "ExtraData text", entry: append(cborText("ExtraData"), 0x61, 'x'), wantErr: "ExtraData"},
		{
			name:    "ExtraData short felt",
			entry:   append(cborText("ExtraData"), 0x83, 1, 2, 3),
			wantErr: "ExtraData",
		},
		{name: "unknown field", entry: append(cborText("Unknown"), 0xf6), wantErr: "Unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var header *core.Header
			err := ugorji.Unmarshal(legacyHeader(t, tc.entry), &header)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			expected := populatedHeader()
			require.Equal(t, &expected, header)
		})
	}
}
