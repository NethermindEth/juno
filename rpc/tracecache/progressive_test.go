package tracecache_test

import (
	"errors"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/tracecache"
	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestResumeStateAppliesCachedChanges(t *testing.T) {
	address := felt.FromUint64[felt.Felt](1)
	key := felt.FromUint64[felt.Felt](2)
	value := felt.FromUint64[felt.Felt](3)
	nonce := felt.FromUint64[felt.Felt](4)
	classHash := felt.FromUint64[felt.Felt](5)
	compiledHash := felt.FromUint64[felt.Felt](6)
	replacement := felt.FromUint64[felt.Felt](7)
	migratedClass := felt.FromUint64[felt.SierraClassHash](8)
	migratedCompiled := felt.FromUint64[felt.CasmClassHash](9)
	replacedAddress := felt.FromUint64[felt.Felt](10)
	legacyHash := felt.FromUint64[felt.Felt](11)

	diff := vm.StateDiff{
		StorageDiffs: []vm.StorageDiff{{
			Address: address, StorageEntries: []vm.Entry{{Key: key, Value: value}},
		}},
		Nonces:                    []vm.Nonce{{ContractAddress: address, Nonce: nonce}},
		DeployedContracts:         []vm.DeployedContract{{Address: address, ClassHash: classHash}},
		DeprecatedDeclaredClasses: []*felt.Felt{&legacyHash},
		DeclaredClasses: []vm.DeclaredClass{{
			ClassHash: classHash, CompiledClassHash: compiledHash,
		}},
		ReplacedClasses: []vm.ReplacedClass{{ContractAddress: replacedAddress, ClassHash: replacement}},
		MigratedCompiledClasses: []vm.MigratedCompiledClass{{
			ClassHash: migratedClass, CompiledClassHash: migratedCompiled,
		}},
	}
	txs := []core.Transaction{
		&core.InvokeTransaction{TransactionHash: &felt.One},
		&core.InvokeTransaction{TransactionHash: felt.NewFromUint64[felt.Felt](2)},
	}
	result := vm.ExecutionResults{
		Traces:      []vm.TransactionTrace{{StateDiff: &diff}},
		GasConsumed: make([]core.GasConsumed, 1),
	}
	prefix, err := tracecache.FromVM(txs[:1], &result, false)
	require.NoError(t, err)
	plan, err := tracecache.PlanRange(prefix, txs, nil, false)
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	parent := mocks.NewMockStateReader(ctrl)
	classes := mocks.NewMockStateReader(ctrl)
	legacyClass := &core.DeprecatedCairoClass{}
	sierraClass := &core.SierraClass{}
	classes.EXPECT().Class(&legacyHash).Return(&core.DeclaredClassDefinition{
		Class: legacyClass,
	}, nil).AnyTimes()
	classes.EXPECT().Class(&classHash).Return(&core.DeclaredClassDefinition{
		Class: sierraClass,
	}, nil).AnyTimes()
	state, err := plan.ResumeState(parent, classes, 10)
	require.NoError(t, err)

	storage, err := state.ContractStorage(&address, &key)
	require.NoError(t, err)
	require.Equal(t, value, storage)
	gotNonce, err := state.ContractNonce(&address)
	require.NoError(t, err)
	require.Equal(t, nonce, gotNonce)
	deployed, err := state.ContractClassHash(&address)
	require.NoError(t, err)
	require.Equal(t, classHash, deployed)
	legacy, err := state.Class(&legacyHash)
	require.NoError(t, err)
	require.Same(t, legacyClass, legacy.Class)
	sierra, err := state.Class(&classHash)
	require.NoError(t, err)
	require.Same(t, sierraClass, sierra.Class)
	sierraHash := felt.SierraClassHash(classHash)
	compiled, err := state.CompiledClassHash(&sierraHash)
	require.NoError(t, err)
	require.Equal(t, felt.CasmClassHash(compiledHash), compiled)
	replaced, err := state.ContractClassHash(&replacedAddress)
	require.NoError(t, err)
	require.Equal(t, replacement, replaced)
	migrated, err := state.CompiledClassHashV2(&migratedClass)
	require.NoError(t, err)
	require.Equal(t, migratedCompiled, migrated)

	diff.StorageDiffs[0].StorageEntries[0].Value.SetUint64(99)
	storage, err = state.ContractStorage(&address, &key)
	require.NoError(t, err)
	require.Equal(t, value, storage)

	t.Run("class lookup failure", func(t *testing.T) {
		lookupErr := errors.New("class unavailable")
		unavailable := mocks.NewMockStateReader(gomock.NewController(t))
		unavailable.EXPECT().Class(gomock.Any()).Return(nil, lookupErr).AnyTimes()
		state, err := plan.ResumeState(parent, unavailable, 10)
		require.ErrorIs(t, err, lookupErr)
		require.Nil(t, state)
	})
}

func TestResumeStateAppliesDiffsInOrder(t *testing.T) {
	address := felt.FromUint64[felt.Felt](1)
	key := felt.FromUint64[felt.Felt](2)
	untouchedKey := felt.FromUint64[felt.Felt](3)
	inheritedKey := felt.FromUint64[felt.Felt](4)
	diffs := []vm.StateDiff{
		{
			StorageDiffs: []vm.StorageDiff{{
				Address: address,
				StorageEntries: []vm.Entry{
					{Key: key, Value: felt.FromUint64[felt.Felt](5)},
					{Key: untouchedKey, Value: felt.FromUint64[felt.Felt](6)},
				},
			}},
			Nonces: []vm.Nonce{{ContractAddress: address, Nonce: felt.One}},
		},
		{
			StorageDiffs: []vm.StorageDiff{{
				Address: address,
				StorageEntries: []vm.Entry{
					{Key: key, Value: felt.FromUint64[felt.Felt](7)},
				},
			}},
			Nonces: []vm.Nonce{{ContractAddress: address, Nonce: felt.FromUint64[felt.Felt](2)}},
		},
	}
	txs := []core.Transaction{
		&core.InvokeTransaction{TransactionHash: &felt.One},
		&core.InvokeTransaction{TransactionHash: felt.NewFromUint64[felt.Felt](2)},
		&core.InvokeTransaction{TransactionHash: felt.NewFromUint64[felt.Felt](3)},
	}
	result := vm.ExecutionResults{
		Traces:      []vm.TransactionTrace{{StateDiff: &diffs[0]}, {StateDiff: &diffs[1]}},
		GasConsumed: make([]core.GasConsumed, 2),
	}
	prefix, err := tracecache.FromVM(txs[:2], &result, false)
	require.NoError(t, err)
	plan, err := tracecache.PlanRange(prefix, txs, nil, false)
	require.NoError(t, err)
	parent := mocks.NewMockStateReader(gomock.NewController(t))
	parent.EXPECT().ContractStorage(&address, &inheritedKey).Return(felt.One, nil).AnyTimes()
	state, err := plan.ResumeState(parent, nil, 10)
	require.NoError(t, err)

	storage, err := state.ContractStorage(&address, &key)
	require.NoError(t, err)
	require.Equal(t, felt.FromUint64[felt.Felt](7), storage)
	untouched, err := state.ContractStorage(&address, &untouchedKey)
	require.NoError(t, err)
	require.Equal(t, felt.FromUint64[felt.Felt](6), untouched)
	inherited, err := state.ContractStorage(&address, &inheritedKey)
	require.NoError(t, err)
	require.Equal(t, felt.One, inherited)
	nonce, err := state.ContractNonce(&address)
	require.NoError(t, err)
	require.Equal(t, felt.FromUint64[felt.Felt](2), nonce)
}

func TestRangeCoverageAndImmutableCombination(t *testing.T) {
	txs := []core.Transaction{
		&core.InvokeTransaction{TransactionHash: felt.NewFromUint64[felt.Felt](1)},
		&core.InvokeTransaction{TransactionHash: felt.NewFromUint64[felt.Felt](2)},
		&core.InvokeTransaction{TransactionHash: felt.NewFromUint64[felt.Felt](3)},
	}
	execute := func(txs []core.Transaction) *tracecache.BlockTrace {
		result := vm.ExecutionResults{
			Traces:      make([]vm.TransactionTrace, len(txs)),
			GasConsumed: make([]core.GasConsumed, len(txs)),
		}
		for i := range txs {
			result.Traces[i].StateDiff = &vm.StateDiff{}
		}
		block, err := tracecache.FromVM(txs, &result, false)
		require.NoError(t, err)
		return block
	}
	target := &tracecache.TransactionTarget{Index: 0, Hash: txs[0].Hash()}
	first, err := tracecache.PlanRange(nil, txs, target, false)
	require.NoError(t, err)
	require.Zero(t, first.Start)
	require.Equal(t, uint64(1), first.End)
	parent := mocks.NewMockStateReader(gomock.NewController(t))
	state, err := first.ResumeState(parent, nil, 10)
	require.NoError(t, err)
	require.Same(t, parent, state)
	prefix, err := first.Combine(execute(txs[:1]))
	require.NoError(t, err)
	require.False(t, prefix.Complete)
	require.False(t, prefix.Covers(false))
	require.True(t, prefix.CoversTarget(target, false))
	// Spare capacity must not allow an extension to mutate previously published storage.
	backing := make([]tracecache.TransactionTrace, 3)
	copy(backing, prefix.Traces)
	prefix.Traces = backing[:1]
	final, err := tracecache.PlanRange(prefix, txs, nil, false)
	require.NoError(t, err)
	require.Equal(t, uint64(1), final.Start)
	require.Equal(t, uint64(3), final.End)
	complete, err := final.Combine(execute(txs[1:]))
	require.NoError(t, err)
	require.Len(t, prefix.Traces, 1)
	require.Nil(t, backing[1].VMTrace())
	require.Len(t, complete.Traces, 3)
	require.True(t, complete.Complete)
	require.True(t, complete.Covers(false))
	require.False(t, complete.Covers(true))
	replay, err := tracecache.PlanRange(prefix, txs, nil, true)
	require.NoError(t, err)
	require.Zero(t, replay.Start)
	require.Equal(t, uint64(3), replay.End)
	state, err = replay.ResumeState(parent, nil, 10)
	require.NoError(t, err)
	require.Same(t, parent, state)
	for _, bad := range []*tracecache.TransactionTarget{
		{Index: 3, Hash: &felt.One},
		{Index: 0, Hash: &felt.Zero},
		{Index: 0},
	} {
		require.ErrorIs(t, complete.ValidateTarget(bad), tracecache.ErrTargetNotFound)
		_, err := tracecache.PlanRange(nil, txs, bad, false)
		require.ErrorIs(t, err, tracecache.ErrTargetNotFound)
	}
	bad := execute(txs[1:])
	bad.Traces[0].VMTrace().StateDiff = nil
	_, err = final.Combine(bad)
	require.EqualError(t, err, "VM omitted state diff for transaction trace 1")
	_, err = tracecache.PlanRange(nil, txs, target, true)
	require.Error(t, err)
	empty, err := tracecache.PlanRange(nil, nil, nil, false)
	require.NoError(t, err)
	result, err := empty.Combine(execute(nil))
	require.NoError(t, err)
	require.True(t, result.Complete)
	require.NotNil(t, result.Traces)
	require.Nil(t, result.InitialReads)
}
