package cbor_test

import (
	"bytes"
	"encoding/binary"
	"iter"
	"slices"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
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

	// Transactions are written behind their type's tag.
	invoke, err := cbor.Marshal(&core.InvokeTransaction{})
	require.NoError(t, err)
	tag := tagHead(invoke)
	require.NotNil(t, tag)
	putBlockTransactions(t, store, 1, []cbor.RawMessage{
		append(bytes.Clone(tag), cborMap(cborText("TransactionHash"), hash)...),
	}, []cbor.RawMessage{
		cborMap(cborText("TransactionHash"), hash),
	})
	expectedTransaction := &core.InvokeTransaction{TransactionHash: &felt.Felt{1, 2, 3, 4}}
	transaction, err := core.GetTransactionByBlockAndIndex(store, 1, 0)
	require.NoError(t, err)
	require.Equal(t, expectedTransaction, transaction)
	transactions, err := core.GetTransactionsByBlockNumber(store, 1)
	require.NoError(t, err)
	require.Equal(t, []core.Transaction{expectedTransaction}, transactions)

	expectedReceipt := &core.TransactionReceipt{TransactionHash: &felt.Felt{1, 2, 3, 4}}
	receipt, err := core.GetReceiptByBlockAndIndex(store, 1, 0)
	require.NoError(t, err)
	require.Equal(t, expectedReceipt, receipt)
	receipts, err := core.GetReceiptsByBlockNumber(store, 1)
	require.NoError(t, err)
	require.Equal(t, []*core.TransactionReceipt{expectedReceipt}, receipts)
}

// Sierra classes are read with Ugorji through the declared class envelope;
// deprecated Cairo classes are not.
func TestClassReadsUseUgorji(t *testing.T) {
	store := memory.New()
	hash := []byte{0x84, 0xf9, 0x3c, 0, 2, 3, 4} // first limb is float16 1.0
	put := func(classHash *felt.Felt, class core.ClassDefinition, content []byte) {
		// The class is written behind its type's tag, after the declaration height.
		tagged, err := cbor.Marshal(class)
		require.NoError(t, err)
		payload := binary.BigEndian.AppendUint64(nil, 7)
		payload = append(append(payload, tagHead(tagged)...), content...)
		stored, err := cbor.Marshal(payload)
		require.NoError(t, err)
		require.NoError(t, store.Put(db.ClassKey(classHash), stored))
	}
	sierraHash, cairoHash := felt.NewFromUint64[felt.Felt](1), felt.NewFromUint64[felt.Felt](2)
	put(sierraHash, &core.SierraClass{}, cborMap(cborText("AbiHash"), hash))
	put(cairoHash, &core.DeprecatedCairoClass{}, cborMap(
		cborText("Externals"), append([]byte{0x81}, cborMap(cborText("Selector"), hash)...),
	))

	expected := &core.DeclaredClassDefinition{
		At:    7,
		Class: &core.SierraClass{AbiHash: &felt.Felt{1, 2, 3, 4}},
	}
	class, err := core.GetClass(store, sierraHash)
	require.NoError(t, err)
	require.Equal(t, expected, class)
	class, err = state.GetClass(store, sierraHash)
	require.NoError(t, err)
	require.Equal(t, expected, class)

	// Only Ugorji accepts this Cairo class, so reading it fails.
	_, err = core.GetClass(store, cairoHash)
	require.Error(t, err)
}
