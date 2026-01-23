package core_test

import (
	"iter"
	"testing"

	"github.com/NethermindEth/juno/adapters/testutils"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/indexed"
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
}
