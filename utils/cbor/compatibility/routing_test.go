package cbor_test

import (
	"bytes"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/NethermindEth/juno/utils/cbor/ugorji"
	"github.com/stretchr/testify/require"
)

func cborText(s string) []byte { return append([]byte{0x60 | byte(len(s))}, s...) }

func cborMap(entries ...[]byte) []byte {
	return append([]byte{0xa0 | byte(len(entries)/2)}, bytes.Join(entries, nil)...)
}

// Storage accessors must reach Ugorji. Each record holds an encoding only Ugorji
// accepts, so the read succeeds only if it was routed there.
func TestAccessorsUseUgorji(t *testing.T) {
	store := memory.New()
	require.NoError(t, store.Put(db.BlockHeaderByNumberKey(1),
		cborMap(cborText("Number"), []byte{0xf9, 0x3c, 0}))) // float16 1.0
	header, err := core.GetBlockHeaderByNumber(store, 1)
	require.NoError(t, err)
	require.Equal(t, &core.Header{Number: 1}, header)

	blockHash := []byte{0x84, 0xf9, 0x3c, 0, 2, 3, 4} // first limb is float16 1.0
	require.NoError(t, store.Put(db.StateUpdateByBlockNumKey(1),
		cborMap(cborText("BlockHash"), blockHash)))
	stateUpdate, err := core.GetStateUpdateByBlockNum(store, 1)
	require.NoError(t, err)
	require.Equal(t, &core.StateUpdate{BlockHash: &felt.Felt{1, 2, 3, 4}}, stateUpdate)
}

// legacyHeader rewrites a header with field names older databases still hold:
// "GasPrice" and "GasPriceSTRK" before #2335. A non-nil entry is appended to
// the map.
func legacyHeader(t *testing.T, header *core.Header, entry []byte) []byte {
	t.Helper()
	data, err := cbor.Marshal(header)
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

// Ugorji reads legacy header fields, including "ExtraData" from before #1498,
// without losing the L1 gas prices, and still rejects unknown fields.
func TestLegacyHeaderFields(t *testing.T) {
	for _, tc := range []struct {
		name    string
		entry   []byte
		wantErr bool
	}{
		{name: "gas prices"},
		{name: "ExtraData null", entry: append(cborText("ExtraData"), 0xf6)},
		{name: "ExtraData felt", entry: append(cborText("ExtraData"), 0x84, 1, 2, 3, 4)},
		{name: "unknown field", entry: append(cborText("Unknown"), 0xf6), wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			expected := populatedHeader()
			data := legacyHeader(t, &expected, tc.entry)

			var header *core.Header
			err := ugorji.Unmarshal(data, &header)
			if tc.wantErr {
				require.ErrorContains(t, err, "Unknown")
				return
			}
			require.NoError(t, err)
			require.Equal(t, &expected, header)

			header = nil
			require.NoError(t, cbor.Unmarshal(data, &header))
			require.Equal(t, &expected, header)
		})
	}
}
