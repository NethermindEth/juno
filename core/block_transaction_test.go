package core_test

import (
	cryptorand "crypto/rand"
	"iter"
	"math/rand/v2"
	"testing"

	"github.com/NethermindEth/juno/adapters/testutils"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/indexed"
	"github.com/NethermindEth/juno/encoder"
	_ "github.com/NethermindEth/juno/encoder/registry"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/require"
)

const sliceLen = 5

func randomEnum[T any](values ...T) T {
	return values[rand.IntN(len(values))]
}

func randomSlice[T any](size int, generator func() T) []T {
	items := make([]T, size)
	for i := range size {
		items[i] = generator()
	}
	return items
}

func randomSliceT[T any](t *testing.T, size int, generator func(t *testing.T) T) []T {
	t.Helper()
	items := make([]T, size)
	for i := range size {
		items[i] = generator(t)
	}
	return items
}

func randomL1Address(t *testing.T) common.Address {
	t.Helper()
	var l1Address common.Address
	read, err := cryptorand.Read(l1Address[:])
	require.Equal(t, len(l1Address), read)
	require.NoError(t, err)
	return l1Address
}

func randomEvent() *core.Event {
	return &core.Event{
		Data: randomSlice(sliceLen, felt.NewRandom[felt.Felt]),
		From: felt.NewRandom[felt.Felt](),
		Keys: randomSlice(sliceLen, felt.NewRandom[felt.Felt]),
	}
}

func randomExecutionResources() *core.ExecutionResources {
	return &core.ExecutionResources{
		BuiltinInstanceCounter: core.BuiltinInstanceCounter{
			Pedersen:     rand.Uint64(),
			RangeCheck:   rand.Uint64(),
			Bitwise:      rand.Uint64(),
			Output:       rand.Uint64(),
			Ecsda:        rand.Uint64(),
			EcOp:         rand.Uint64(),
			Keccak:       rand.Uint64(),
			Poseidon:     rand.Uint64(),
			SegmentArena: rand.Uint64(),
			AddMod:       rand.Uint64(),
			MulMod:       rand.Uint64(),
			RangeCheck96: rand.Uint64(),
		},
		MemoryHoles: rand.Uint64(),
		Steps:       rand.Uint64(),
		DataAvailability: &core.DataAvailability{
			L1Gas:     rand.Uint64(),
			L1DataGas: rand.Uint64(),
		},
		TotalGasConsumed: &core.GasConsumed{
			L1Gas:     rand.Uint64(),
			L1DataGas: rand.Uint64(),
			L2Gas:     rand.Uint64(),
		},
	}
}

func randomL1ToL2Message(t *testing.T) *core.L1ToL2Message {
	t.Helper()
	return &core.L1ToL2Message{
		From:     randomL1Address(t),
		Nonce:    felt.NewRandom[felt.Felt](),
		Payload:  randomSlice(sliceLen, felt.NewRandom[felt.Felt]),
		Selector: felt.NewRandom[felt.Felt](),
		To:       felt.NewRandom[felt.Felt](),
	}
}

func randomL2ToL1Message(t *testing.T) *core.L2ToL1Message {
	t.Helper()
	return &core.L2ToL1Message{
		From:    felt.NewRandom[felt.Felt](),
		Payload: randomSlice(sliceLen, felt.NewRandom[felt.Felt]),
		To:      randomL1Address(t),
	}
}

func randomReceipt(t *testing.T) *core.TransactionReceipt {
	t.Helper()
	return &core.TransactionReceipt{
		Fee:                felt.NewRandom[felt.Felt](),
		FeeUnit:            randomEnum(core.STRK, core.WEI),
		Events:             randomSlice(sliceLen, randomEvent),
		ExecutionResources: randomExecutionResources(),
		L1ToL2Message:      randomL1ToL2Message(t),
		L2ToL1Message:      randomSliceT(t, sliceLen, randomL2ToL1Message),
		TransactionHash:    felt.NewRandom[felt.Felt](),
		Reverted:           rand.IntN(2) == 0,
		RevertReason:       cryptorand.Text(),
	}
}

func getTransactionsAndReceipts(t *testing.T) ([]core.Transaction, []*core.TransactionReceipt) {
	transactionBuilder := testutils.SyncTransactionBuilder[core.Transaction, struct{}]{
		ToCore: func(
			transaction core.Transaction,
			class core.ClassDefinition,
			paidFeeOnL1 *felt.Felt,
		) core.Transaction {
			return transaction
		},
	}
	transactions, _ := testutils.GetTestTransactions(
		t,
		&utils.Mainnet,
		transactionBuilder.GetTestDeclareV0Transaction,
		transactionBuilder.GetTestDeclareV1Transaction,
		transactionBuilder.GetTestDeclareV2Transaction,
		transactionBuilder.GetTestDeclareV3Transaction,
		transactionBuilder.GetTestDeployTransactionV0,
		transactionBuilder.GetTestDeployAccountTransactionV1,
		transactionBuilder.GetTestDeployAccountTransactionV3,
		transactionBuilder.GetTestInvokeTransactionV0,
		transactionBuilder.GetTestInvokeTransactionV1,
		transactionBuilder.GetTestInvokeTransactionV3,
		transactionBuilder.GetTestL1HandlerTransaction,
	)
	receipts := randomSliceT(t, len(transactions), randomReceipt)

	return transactions, receipts
}

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
	transactions, receipts := getTransactionsAndReceipts(t)
	blockTransactions, err := core.NewBlockTransactions(transactions, receipts)
	require.NoError(t, err)
	assertBlockTransactions(t, blockTransactions, transactions, receipts)
}

func TestNewBlockTransactionsFromIterators(t *testing.T) {
	transactions, receipts := getTransactionsAndReceipts(t)
	blockTransactions, err := core.NewBlockTransactionsFromIterators(
		toCborSeq(transactions),
		toCborSeq(receipts),
	)
	require.NoError(t, err)
	assertBlockTransactions(t, blockTransactions, transactions, receipts)
}

func TestBlockTransactionsSerializer(t *testing.T) {
	transactions, receipts := getTransactionsAndReceipts(t)
	blockTransactions, err := core.NewBlockTransactions(transactions, receipts)
	require.NoError(t, err)

	serialised, err := core.BlockTransactionsSerializer{}.Marshal(&blockTransactions)
	require.NoError(t, err)

	var deserialized core.BlockTransactions
	require.NoError(t, core.BlockTransactionsSerializer{}.Unmarshal(serialised, &deserialized))

	assertBlockTransactions(t, deserialized, transactions, receipts)
}
