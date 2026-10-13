package records_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/stretchr/testify/require"
)

// BenchmarkUnmarshal decodes each fixture through the wrapper into a fresh destination, as the
// database reads do.
func BenchmarkUnmarshal(b *testing.B) {
	benchmarkUnmarshal[core.TransactionReceipt](b, "receipt", receipt())
	benchmarkUnmarshal[core.Header](b, "header", header())
	class := core.ClassDefinition(sierraClass())
	benchmarkUnmarshal[core.ClassDefinition](b, "sierra-class", &class)
	benchmarkUnmarshal[core.StateUpdate](b, "state-update", stateUpdate())
	txs := transactions()
	for _, name := range slices.Sorted(maps.Keys(txs)) {
		tx := txs[name]
		benchmarkUnmarshal[core.Transaction](b, "transaction/"+name, &tx)
	}
}

func benchmarkUnmarshal[T any](b *testing.B, name string, value any) {
	data, err := cbor.Marshal(value)
	require.NoError(b, err)
	b.Run(name, func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(data)))
		for b.Loop() {
			var dst T
			if err := cbor.Unmarshal(data, &dst); err != nil {
				b.Fatal(err)
			}
		}
	})
}
