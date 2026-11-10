package core_test

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/stretchr/testify/require"
)

// BenchmarkTypedHeader measures the generated decoder directly, without routing
// or fallback. Serialization and the output check are outside the timed loop.
func BenchmarkTypedHeader(b *testing.B) {
	f := felt.NewFromUint64[felt.Felt](1 << 40)
	for _, tc := range []struct {
		name  string
		value core.Header
	}{
		{"zero", core.Header{}},
		{"scalars", core.Header{Number: 1 << 32, TransactionCount: 128, EventCount: 256, Timestamp: 1 << 40, ProtocolVersion: "0.13.α", L1DAMode: core.Blob}},
		{"prices", core.Header{L1DataGasPrice: &core.GasPrice{PriceInWei: f, PriceInFri: f}, L2GasPrice: &core.GasPrice{PriceInWei: f, PriceInFri: f}}},
		{"full", core.Header{Hash: f, ParentHash: f, Number: 1 << 32, GlobalStateRoot: f, SequencerAddress: f, TransactionCount: 128, EventCount: 256, Timestamp: 1 << 40, ProtocolVersion: "0.13.α", L1GasPriceETH: f, Signatures: [][]*felt.Felt{nil, {}, {nil, f, f}}, L1GasPriceSTRK: f, L1DAMode: core.Blob, L1DataGasPrice: &core.GasPrice{PriceInWei: f, PriceInFri: f}, L2GasPrice: &core.GasPrice{PriceInWei: f, PriceInFri: f}}},
	} {
		b.Run(tc.name, func(b *testing.B) {
			data, err := cbor.Marshal(tc.value)
			require.NoError(b, err)
			var got core.Header
			require.NoError(b, core.DecodeHeader(data, &got))
			require.Equal(b, tc.value, got)
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			for b.Loop() {
				got = core.Header{}
				if err := core.DecodeHeader(data, &got); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
