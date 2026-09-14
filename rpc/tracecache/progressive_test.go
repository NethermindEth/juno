package tracecache

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/require"
)

func TestMergeVMStateDiff(t *testing.T) {
	address := felt.FromUint64[felt.Felt](1)
	key := felt.FromUint64[felt.Felt](2)
	value := felt.FromUint64[felt.Felt](3)
	nonce := felt.FromUint64[felt.Felt](4)
	classHash := felt.FromUint64[felt.Felt](5)
	compiledHash := felt.FromUint64[felt.Felt](6)
	replacement := felt.FromUint64[felt.Felt](7)
	migratedClass := felt.FromUint64[felt.SierraClassHash](8)
	migratedCompiled := felt.FromUint64[felt.CasmClassHash](9)

	diff := vm.StateDiff{
		StorageDiffs: []vm.StorageDiff{{
			Address: address, StorageEntries: []vm.Entry{{Key: key, Value: value}},
		}},
		Nonces:                    []vm.Nonce{{ContractAddress: address, Nonce: nonce}},
		DeployedContracts:         []vm.DeployedContract{{Address: address, ClassHash: classHash}},
		DeprecatedDeclaredClasses: []*felt.Felt{&classHash},
		DeclaredClasses: []vm.DeclaredClass{{
			ClassHash: classHash, CompiledClassHash: compiledHash,
		}},
		ReplacedClasses: []vm.ReplacedClass{{ContractAddress: address, ClassHash: replacement}},
		MigratedCompiledClasses: []vm.MigratedCompiledClass{{
			ClassHash: migratedClass, CompiledClassHash: migratedCompiled,
		}},
	}
	converted := core.EmptyStateDiff()
	mergeVMStateDiff(&converted, &diff)

	require.Equal(t, value, *converted.StorageDiffs[address][key])
	require.Equal(t, nonce, *converted.Nonces[address])
	require.Equal(t, classHash, *converted.DeployedContracts[address])
	require.Equal(t, classHash, *converted.DeclaredV0Classes[0])
	require.Equal(t, compiledHash, *converted.DeclaredV1Classes[classHash])
	require.Equal(t, replacement, *converted.ReplacedClasses[address])
	require.Equal(t, migratedCompiled, converted.MigratedClasses[migratedClass])

	diff.StorageDiffs[0].StorageEntries[0].Value.SetUint64(99)
	require.Equal(t, uint64(3), converted.StorageDiffs[address][key].Uint64())
}

func TestRangeCoverageAndImmutableCombination(t *testing.T) {
	txs := []core.Transaction{
		&core.InvokeTransaction{TransactionHash: felt.NewFromUint64[felt.Felt](1)},
		&core.InvokeTransaction{TransactionHash: felt.NewFromUint64[felt.Felt](2)},
		&core.InvokeTransaction{TransactionHash: felt.NewFromUint64[felt.Felt](3)},
	}
	execute := func(txs []core.Transaction) *BlockTrace {
		result := vm.ExecutionResults{
			Traces:      make([]vm.TransactionTrace, len(txs)),
			GasConsumed: make([]core.GasConsumed, len(txs)),
		}
		for i := range txs {
			result.Traces[i].StateDiff = &vm.StateDiff{}
		}
		block, err := FromVM(txs, &result, false)
		require.NoError(t, err)
		return block
	}
	target := &TransactionTarget{Index: 0, Hash: txs[0].Hash()}
	first, err := PlanRange(nil, txs, target, false)
	require.NoError(t, err)
	require.Zero(t, first.Start)
	require.Equal(t, uint64(1), first.End)
	prefix, err := first.Combine(execute(txs[:1]))
	require.NoError(t, err)
	require.False(t, prefix.Complete)
	require.False(t, prefix.Covers(false))
	require.True(t, prefix.CoversTarget(target, false))
	// Spare capacity must not allow an extension to mutate previously published storage.
	backing := make([]TransactionTrace, 3)
	copy(backing, prefix.Traces)
	prefix.Traces = backing[:1]
	final, err := PlanRange(prefix, txs, nil, false)
	require.NoError(t, err)
	require.Equal(t, uint64(1), final.Start)
	require.Equal(t, uint64(3), final.End)
	complete, err := final.Combine(execute(txs[1:]))
	require.NoError(t, err)
	require.Len(t, prefix.Traces, 1)
	require.Nil(t, backing[1].vmTrace)
	require.Len(t, complete.Traces, 3)
	require.True(t, complete.Complete)
	require.True(t, complete.Covers(false))
	require.False(t, complete.Covers(true))
	replay, err := PlanRange(prefix, txs, nil, true)
	require.NoError(t, err)
	require.Zero(t, replay.Start)
	require.Equal(t, uint64(3), replay.End)
	for _, bad := range []*TransactionTarget{
		{Index: 3, Hash: &felt.One},
		{Index: 0, Hash: &felt.Zero},
		{Index: 0},
	} {
		require.ErrorIs(t, complete.ValidateTarget(bad), ErrTargetNotFound)
		_, err := PlanRange(nil, txs, bad, false)
		require.ErrorIs(t, err, ErrTargetNotFound)
	}
	bad := execute(txs[1:])
	bad.Traces[0].vmTrace.StateDiff = nil
	_, err = final.Combine(bad)
	require.EqualError(t, err, "VM omitted state diff for transaction trace 1")
	_, err = PlanRange(nil, txs, target, true)
	require.Error(t, err)
	empty, err := PlanRange(nil, nil, nil, false)
	require.NoError(t, err)
	result, err := empty.Combine(execute(nil))
	require.NoError(t, err)
	require.True(t, result.Complete)
	require.NotNil(t, result.Traces)
	require.Nil(t, result.InitialReads)
}
