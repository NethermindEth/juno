package core

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/encoder"
	"github.com/stretchr/testify/require"
)

func newBound(amount, price uint64) ResourceBounds {
	return ResourceBounds{MaxAmount: amount, MaxPricePerUnit: new(felt.Felt).SetUint64(price)}
}

// TestResourceBoundsMapCBORCompat pins the on-disk format: ResourceBoundsMap
// must serialise to, and deserialise from, the exact CBOR bytes that the legacy
// map[Resource]ResourceBounds field produced, so transactions stored before this
// refactor stay readable without a DB migration.
func TestResourceBoundsMapCBORCompat(t *testing.T) {
	l1 := newBound(1, 2)
	l2 := newBound(3, 4)
	l1data := newBound(5, 6)

	tests := []struct {
		name   string
		legacy map[Resource]ResourceBounds
		value  ResourceBoundsMap
	}{
		{
			name: "all three resources",
			legacy: map[Resource]ResourceBounds{
				ResourceL1Gas: l1, ResourceL2Gas: l2, ResourceL1DataGas: l1data,
			},
			value: ResourceBoundsMap{L1Gas: l1, L2Gas: l2, L1DataGas: l1data},
		},
		{
			name:   "no l1_data_gas (pre-0.13.4 v3)",
			legacy: map[Resource]ResourceBounds{ResourceL1Gas: l1, ResourceL2Gas: l2},
			value:  ResourceBoundsMap{L1Gas: l1, L2Gas: l2},
		},
		{
			name:   "nil map (v1/v2)",
			legacy: nil,
			value:  ResourceBoundsMap{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			legacyBytes, err := encoder.Marshal(tt.legacy)
			require.NoError(t, err)

			valueBytes, err := encoder.Marshal(tt.value)
			require.NoError(t, err)
			require.Equal(t, legacyBytes, valueBytes, "struct must encode to the legacy map bytes")

			var decoded ResourceBoundsMap
			require.NoError(t, encoder.Unmarshal(legacyBytes, &decoded))
			require.Equal(t, tt.value, decoded, "legacy bytes must decode into the struct")
		})
	}
}

// TestTipAndResourcesHashL1DataGasPresence guards that l1_data_gas only enters
// the transaction hash when it is present (non-nil MaxPricePerUnit), preserving
// the pre-0.13.4 behaviour the old map-key check provided.
func TestTipAndResourcesHashL1DataGasPresence(t *testing.T) {
	const tip = 7
	l1 := newBound(1, 2)
	l2 := newBound(3, 4)

	// L1DataGas is left zero, so its MaxPricePerUnit stays nil (absent).
	absent := ResourceBoundsMap{L1Gas: l1, L2Gas: l2}
	// A present but zero-valued l1_data_gas still enters the hash.
	present := ResourceBoundsMap{L1Gas: l1, L2Gas: l2, L1DataGas: newBound(0, 0)}

	absentHash := tipAndResourcesHash(tip, absent)
	require.NotEqual(t, absentHash, tipAndResourcesHash(tip, present),
		"a present l1_data_gas must change the hash")

	// The absent case is independent of the (zero) L1DataGas fields.
	absentExplicitZero := ResourceBoundsMap{L1Gas: l1, L2Gas: l2, L1DataGas: ResourceBounds{}}
	require.Equal(t, absentHash, tipAndResourcesHash(tip, absentExplicitZero))
}
