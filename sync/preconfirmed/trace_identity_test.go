package preconfirmed_test

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/pending"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/sync/preconfirmed"
	"github.com/stretchr/testify/require"
)

func TestTraceExecutionIdentityUpdates(t *testing.T) {
	classes := map[felt.Felt]core.ClassDefinition{felt.One: nil}
	tests := []struct {
		name    string
		update  starknet.PreConfirmedUpdate
		classes map[felt.Felt]core.ClassDefinition
		changes bool
	}{
		{name: "delta", update: makeTestDelta("round", 1)},
		{name: "no change", update: starknet.PreConfirmedNoChange{}},
		{name: "ignored full", update: makeTestPreConfirmedBlock("round", 1)},
		{name: "replacement", update: makeTestPreConfirmedBlock("replacement", 1), changes: true},
		{name: "richer full", update: makeTestPreConfirmedBlock("round", 2), changes: true},
		{name: "delta classes", update: makeTestDelta("round", 1), classes: classes, changes: true},
		{name: "class only", update: starknet.PreConfirmedNoChange{}, classes: classes, changes: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			storage := preconfirmed.NewChainStorage()
			applyBlock(t, storage, "round", 1, 10, 10)
			before := storage.SnapshotForBlock(10)
			generation, base, err := before.TraceExecutionIdentity(10)
			require.NoError(t, err)
			require.NotZero(t, generation)
			require.Equal(t, uint64(9), base)
			_, err = storage.ApplyUpdate(test.update, 10, 1, 10, test.classes)
			require.NoError(t, err)
			after := storage.SnapshotForBlock(10)
			next, _, err := after.TraceExecutionIdentity(10)
			require.NoError(t, err)
			require.Equal(t, test.changes, generation != next)
			pinned, _, err := before.TraceExecutionIdentity(10)
			require.NoError(t, err)
			require.Equal(t, generation, pinned)
		})
	}
}

func TestTraceExecutionIdentityTrim(t *testing.T) {
	storage := preconfirmed.NewChainStorage()
	applyBlock(t, storage, "first", 1, 10, 10)
	applyBlock(t, storage, "second", 1, 11, 10)
	both := storage.SnapshotForBlock(10)
	generation, base, err := both.TraceExecutionIdentity(11)
	require.NoError(t, err)
	require.Equal(t, uint64(9), base)

	// A view trim and a physical trim preserve the generation but change the base.
	trimmed := storage.SnapshotForBlock(11)
	next, base, err := trimmed.TraceExecutionIdentity(11)
	require.NoError(t, err)
	require.Equal(t, generation, next)
	require.Equal(t, uint64(10), base)
	require.True(t, storage.AdvanceTo(11))
	rebuilt := storage.SnapshotForBlock(11)
	next, base, err = rebuilt.TraceExecutionIdentity(11)
	require.NoError(t, err)
	require.Equal(t, generation, next)
	require.Equal(t, uint64(10), base)
}

func TestTraceExecutionIdentityIndependentChains(t *testing.T) {
	var empty preconfirmed.ChainReader
	_, _, err := empty.TraceExecutionIdentity(1)
	require.ErrorIs(t, err, pending.ErrPreConfirmedNotFound)
	entry := &pending.PreConfirmed{Block: &core.Block{Header: &core.Header{Number: 1}}}
	first, err := preconfirmed.NewChain(entry)
	require.NoError(t, err)
	second, err := preconfirmed.NewChain(entry)
	require.NoError(t, err)
	a, _, err := first.TraceExecutionIdentity(1)
	require.NoError(t, err)
	b, _, err := second.TraceExecutionIdentity(1)
	require.NoError(t, err)
	require.NotEqual(t, a, b)
}
