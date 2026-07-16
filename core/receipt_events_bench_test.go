package core_test

import (
	"testing"

	"github.com/NethermindEth/juno/adapters/testutils"
	"github.com/NethermindEth/juno/core"
	"github.com/stretchr/testify/require"
)

// Compares full-receipt decode against the reduced events-only view over the
// same stored block bytes.
func BenchmarkReceiptDecode(b *testing.B) {
	receipts := testutils.GetCoreReceipts(b, transactionCount)
	bt, err := core.NewBlockTransactions(nil, receipts)
	require.NoError(b, err)

	b.Run("FullReceipts", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := bt.Receipts().All(); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("EventsOnly", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := bt.ReceiptEvents().All(); err != nil {
				b.Fatal(err)
			}
		}
	})
}
