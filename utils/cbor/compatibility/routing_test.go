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
// "GasPrice" and "GasPriceSTRK" before #2335, and "ExtraData" before #1498.
func legacyHeader(t *testing.T, header *core.Header, extraData bool) []byte {
	t.Helper()
	data, err := cbor.Marshal(header)
	require.NoError(t, err)

	for _, rename := range [][2]string{{"gasprice", "GasPrice"}, {"gaspricestrk", "GasPriceSTRK"}} {
		current, legacy := cborText(rename[0]), cborText(rename[1])
		require.Equal(t, 1, bytes.Count(data, current), rename[0])
		data = bytes.Replace(data, current, legacy, 1)
	}
	if extraData {
		// The field count fits in the map's initial byte; append one more entry.
		require.Equal(t, byte(0xa0), data[0]&0xe0)
		require.Less(t, data[0]&0x1f, byte(23))
		data[0]++
		data = append(append(data, cborText("ExtraData")...), 0xf6)
	}
	return data
}

// Ugorji rejects legacy field names, so these reads must fall back without
// losing the L1 gas prices.
func TestLegacyHeaderFieldsFallBack(t *testing.T) {
	for _, extraData := range []bool{false, true} {
		expected := populatedHeader()
		data := legacyHeader(t, &expected, extraData)

		require.Error(t, ugorji.Unmarshal(data, new(*core.Header)))
		var header *core.Header
		require.NoError(t, cbor.Unmarshal(data, &header))
		require.Equal(t, &expected, header)
	}
}
