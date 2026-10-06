package cbor_test

import (
	"bytes"
	"iter"
	"slices"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/stretchr/testify/require"
)

func cborMap(entries ...[]byte) []byte {
	return append([]byte{0xa0 | byte(len(entries)/2)}, bytes.Join(entries, nil)...)
}

// putBlockTransactions stores already encoded transactions and receipts as a block's entry.
func putBlockTransactions(
	t *testing.T,
	store db.KeyValueWriter,
	number uint64,
	transactions, receipts []cbor.RawMessage,
) {
	t.Helper()
	encoded := func(items []cbor.RawMessage) iter.Seq2[cbor.RawMessage, error] {
		return func(yield func(cbor.RawMessage, error) bool) {
			for item := range slices.Values(items) {
				if !yield(item, nil) {
					return
				}
			}
		}
	}
	blockTransactions, err := core.NewBlockTransactionsFromIterators(
		encoded(transactions), encoded(receipts),
	)
	require.NoError(t, err)
	require.NoError(t, core.BlockTransactionsBucket.Put(store, number, &blockTransactions))
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

	hash := []byte{0x84, 0xf9, 0x3c, 0, 2, 3, 4} // first limb is float16 1.0
	require.NoError(t, store.Put(db.StateUpdateByBlockNumKey(1),
		cborMap(cborText("BlockHash"), hash)))
	stateUpdate, err := core.GetStateUpdateByBlockNum(store, 1)
	require.NoError(t, err)
	require.Equal(t, &core.StateUpdate{BlockHash: &felt.Felt{1, 2, 3, 4}}, stateUpdate)

	putBlockTransactions(t, store, 1, nil, []cbor.RawMessage{
		cborMap(cborText("TransactionHash"), hash),
	})
	expectedReceipt := &core.TransactionReceipt{TransactionHash: &felt.Felt{1, 2, 3, 4}}
	receipt, err := core.GetReceiptByBlockAndIndex(store, 1, 0)
	require.NoError(t, err)
	require.Equal(t, expectedReceipt, receipt)
	receipts, err := core.GetReceiptsByBlockNumber(store, 1)
	require.NoError(t, err)
	require.Equal(t, []*core.TransactionReceipt{expectedReceipt}, receipts)
}
