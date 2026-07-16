package core_test

import (
	"iter"
	"testing"

	"github.com/NethermindEth/juno/adapters/testutils"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/indexed"
	"github.com/NethermindEth/juno/db/typed/partial"
	"github.com/NethermindEth/juno/encoder"
	_ "github.com/NethermindEth/juno/encoder/registry"
	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/require"
)

const transactionCount = 100

func toCborSeq[T any](items []T) iter.Seq2[cbor.RawMessage, error] {
	return func(yield func(cbor.RawMessage, error) bool) {
		for _, item := range items {
			cbor, err := encoder.Marshal(item)
			if err != nil {
				yield(nil, err)
				return
			}

			if !yield(cbor, nil) {
				return
			}
		}
	}
}

func assertLazySlice[T any](t *testing.T, expected []T, lazySlice indexed.LazySlice[T]) {
	t.Helper()

	actual, err := lazySlice.All()
	require.NoError(t, err)
	require.Equal(t, expected, actual)

	for i, expectedItem := range expected {
		actualItem, err := lazySlice.Get(i)
		require.NoError(t, err)
		require.Equal(t, expectedItem, actualItem)
	}
}

func assertBlockTransactions(
	t *testing.T,
	blockTransactions core.BlockTransactions,
	expectedTransactions []core.Transaction,
	expectedReceipts []*core.TransactionReceipt,
) {
	t.Helper()
	require.Len(t, blockTransactions.Indexes.Transactions, len(expectedTransactions))
	require.Len(t, blockTransactions.Indexes.Receipts, len(expectedReceipts))
	assertLazySlice(t, expectedTransactions, blockTransactions.Transactions())
	assertLazySlice(t, expectedReceipts, blockTransactions.Receipts())
}

func TestNewBlockTransactions(t *testing.T) {
	transactions := testutils.GetCoreTransactions(t, transactionCount)
	receipts := testutils.GetCoreReceipts(t, transactionCount)
	blockTransactions, err := core.NewBlockTransactions(transactions, receipts)
	require.NoError(t, err)
	assertBlockTransactions(t, blockTransactions, transactions, receipts)
}

func TestNewBlockTransactionsFromIterators(t *testing.T) {
	transactions := testutils.GetCoreTransactions(t, transactionCount)
	receipts := testutils.GetCoreReceipts(t, transactionCount)
	blockTransactions, err := core.NewBlockTransactionsFromIterators(
		toCborSeq(transactions),
		toCborSeq(receipts),
	)
	require.NoError(t, err)
	assertBlockTransactions(t, blockTransactions, transactions, receipts)
}

// Pins that the reduced ReceiptEvents view decodes the same events and
// transaction hash as the full receipts from the same stored bytes. Guards the
// CBOR field-name alignment between ReceiptEvents and TransactionReceipt.
func TestReceiptEvents_MatchesReceipt(t *testing.T) {
	transactions := testutils.GetCoreTransactions(t, transactionCount)
	receipts := testutils.GetCoreReceipts(t, transactionCount)
	blockTransactions, err := core.NewBlockTransactions(transactions, receipts)
	require.NoError(t, err)

	views, err := blockTransactions.ReceiptEvents().All()
	require.NoError(t, err)
	require.Len(t, views, len(receipts))
	for i, r := range receipts {
		require.Equal(t, r.Events, views[i].Events, "receipt %d events", i)
		require.Equal(t, r.TransactionHash, views[i].TransactionHash, "receipt %d hash", i)
	}

	// The in-memory adapter (pre-confirmed path) yields the same views.
	require.Equal(t, views, core.ReceiptEventsFromReceipts(receipts))
}

func assertPartialSerializer[E partial.PartialSerializer[S, T], S, T any](
	t *testing.T,
	serializer E,
	subKey S,
	expected T,
	serialised []byte,
) {
	t.Helper()
	var deserialized T
	require.NoError(t, serializer.UnmarshalPartial(subKey, serialised, &deserialized))
	require.Equal(t, expected, deserialized)
}

func TestBlockTransactionsSerializer(t *testing.T) {
	transactions := testutils.GetCoreTransactions(t, transactionCount)
	receipts := testutils.GetCoreReceipts(t, transactionCount)
	blockTransactions, err := core.NewBlockTransactions(transactions, receipts)
	require.NoError(t, err)

	serialised, err := core.BlockTransactionsSerializer{}.Marshal(&blockTransactions)
	require.NoError(t, err)

	var deserialized core.BlockTransactions
	require.NoError(t, core.BlockTransactionsSerializer{}.Unmarshal(serialised, &deserialized))

	assertBlockTransactions(t, deserialized, transactions, receipts)

	t.Run("BlockTransactionsTransactionPartialSerializer", func(t *testing.T) {
		for i := range transactionCount {
			assertPartialSerializer(
				t,
				core.BlockTransactionsTransactionPartialSerializer,
				i,
				transactions[i],
				serialised,
			)
		}
	})

	t.Run("BlockTransactionsReceiptPartialSerializer", func(t *testing.T) {
		for i := range transactionCount {
			assertPartialSerializer(
				t,
				core.BlockTransactionsReceiptPartialSerializer,
				i,
				receipts[i],
				serialised,
			)
		}
	})

	t.Run("BlockTransactionsAllTransactionsPartialSerializer", func(t *testing.T) {
		assertPartialSerializer(
			t,
			core.BlockTransactionsAllTransactionsPartialSerializer,
			struct{}{},
			transactions,
			serialised,
		)
	})

	t.Run("BlockTransactionsAllReceiptsPartialSerializer", func(t *testing.T) {
		assertPartialSerializer(
			t,
			core.BlockTransactionsAllReceiptsPartialSerializer,
			struct{}{},
			receipts,
			serialised,
		)
	})
}
