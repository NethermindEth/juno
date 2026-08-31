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
	"github.com/NethermindEth/juno/utils/cbor"
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

	t.Run("BlockTransactionsAllTransactionsAndReceiptsPartialSerializer", func(t *testing.T) {
		assertPartialSerializer(
			t,
			core.BlockTransactionsAllTransactionsAndReceiptsPartialSerializer,
			struct{}{},
			core.TransactionsAndReceipts{Transactions: transactions, Receipts: receipts},
			serialised,
		)
	})

	t.Run("BlockTransactionsAllTransactionEventsPartialSerializer", func(t *testing.T) {
		expected := make([]core.TransactionEvents, len(receipts))
		for i, receipt := range receipts {
			expected[i] = core.TransactionEvents{
				Events:          receipt.Events,
				TransactionHash: receipt.TransactionHash,
			}
		}
		assertPartialSerializer(
			t,
			core.BlockTransactionsAllTransactionEventsPartialSerializer,
			struct{}{},
			expected,
			serialised,
		)
	})

	t.Run("BlockTransactionsExecutionStatusPartialSerializer", func(t *testing.T) {
		for i := range transactionCount {
			assertPartialSerializer(
				t,
				core.BlockTransactionsExecutionStatusPartialSerializer,
				i,
				core.TransactionExecutionStatus{
					Reverted:     receipts[i].Reverted,
					RevertReason: receipts[i].RevertReason,
				},
				serialised,
			)
		}
	})

	t.Run("BlockTransactionsAllTransactionHashesPartialSerializer", func(t *testing.T) {
		// The serializer reads each transaction's own hash from the transaction section.
		assertPartialSerializer(
			t,
			core.BlockTransactionsAllTransactionHashesPartialSerializer,
			struct{}{},
			transactionHashesOf(transactions),
			serialised,
		)
	})
}
