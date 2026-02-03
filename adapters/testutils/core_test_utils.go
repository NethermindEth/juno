//nolint:gosec // The whole package is for testing purpose only, so it's safe to use weak random.
package testutils

import (
	cryptorand "crypto/rand"
	"math/rand/v2"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/l1/types"
	"github.com/NethermindEth/juno/utils"
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

func randomL1Address(t *testing.T) *types.L1Address {
	t.Helper()
	addr := types.Random[types.L1Address]()
	return &addr
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
		To:       felt.NewRandom[felt.Address](),
	}
}

func randomL2ToL1Message(t *testing.T) *core.L2ToL1Message {
	t.Helper()
	return &core.L2ToL1Message{
		From:    felt.NewRandom[felt.Address](),
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

func GetCoreTransactions(t *testing.T, count int) []core.Transaction {
	t.Helper()
	transactionBuilder := SyncTransactionBuilder[core.Transaction, struct{}]{
		ToCore: func(
			transaction core.Transaction,
			class core.ClassDefinition,
			paidFeeOnL1 *felt.Felt,
		) core.Transaction {
			return transaction
		},
		ToP2PDeclareV0: nil,
		ToP2PDeclareV1: nil,
		ToP2PDeclareV2: nil,
		ToP2PDeclareV3: nil,
		ToP2PDeployV0:  nil,
		ToP2PDeployV1:  nil,
		ToP2PDeployV3:  nil,
		ToP2PInvokeV0:  nil,
		ToP2PInvokeV1:  nil,
		ToP2PInvokeV3:  nil,
		ToP2PL1Handler: nil,
	}

	return randomSliceT(
		t,
		count,
		func(t *testing.T) core.Transaction {
			generator := randomEnum(
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
			transaction, _ := generator(t, &utils.Mainnet)
			return transaction
		},
	)
}

func GetCoreReceipts(t *testing.T, count int) []*core.TransactionReceipt {
	t.Helper()
	return randomSliceT(t, count, randomReceipt)
}
