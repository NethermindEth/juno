package starknet_test

import (
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/starknet"
	"github.com/stretchr/testify/require"
)

// TestResourceBoundsMapJSON checks that the feeder gateway JSON representation
// is preserved: uppercase L1_GAS/L2_GAS/L1_DATA_GAS keys, round-tripping both
// directions.
func TestResourceBoundsMapJSON(t *testing.T) {
	full := `{"L1_GAS":{"max_amount":"0x1","max_price_per_unit":"0x2"},` +
		`"L2_GAS":{"max_amount":"0x3","max_price_per_unit":"0x4"},` +
		`"L1_DATA_GAS":{"max_amount":"0x5","max_price_per_unit":"0x6"}}`

	var rb starknet.ResourceBoundsMap
	require.NoError(t, json.Unmarshal([]byte(full), &rb))
	require.Equal(t, uint64(1), rb.L1Gas.MaxAmount.Uint64())
	require.Equal(t, uint64(4), rb.L2Gas.MaxPricePerUnit.Uint64())
	require.Equal(t, uint64(5), rb.L1DataGas.MaxAmount.Uint64())

	out, err := json.Marshal(rb)
	require.NoError(t, err)
	require.JSONEq(t, full, string(out))
}

// TestResourceBoundsMapJSONOmitsAbsentL1DataGas checks that a pre-0.13.4 feeder
// response (only L1_GAS and L2_GAS) leaves l1_data_gas zero and does not
// re-introduce the key when marshalled again.
func TestResourceBoundsMapJSONOmitsAbsentL1DataGas(t *testing.T) {
	twoKeys := `{"L1_GAS":{"max_amount":"0x1","max_price_per_unit":"0x2"},` +
		`"L2_GAS":{"max_amount":"0x3","max_price_per_unit":"0x4"}}`

	var rb starknet.ResourceBoundsMap
	require.NoError(t, json.Unmarshal([]byte(twoKeys), &rb))
	require.Nil(t, rb.L1DataGas.MaxAmount)
	require.Nil(t, rb.L1DataGas.MaxPricePerUnit)

	out, err := json.Marshal(rb)
	require.NoError(t, err)
	require.JSONEq(t, twoKeys, string(out))
}
