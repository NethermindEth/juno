package builder_test

import (
	"reflect"
	"testing"

	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/pending"
	"github.com/stretchr/testify/require"
)

// fullStateDiff populates every StateDiff field for a fixed address with values derived from seed.
func fullStateDiff(seed uint64) core.StateDiff {
	address := felt.FromUint64[felt.Address](1)
	classHash := felt.FromUint64[felt.ClassHash](seed)
	sierraClassHash := felt.FromUint64[felt.SierraClassHash](seed)
	casmClassHash := felt.FromUint64[felt.CasmClassHash](seed)
	return core.StateDiff{
		StorageDiffs: map[felt.Address]map[felt.Felt]*felt.Felt{
			address: {
				felt.FromUint64[felt.Felt](seed): felt.NewFromUint64[felt.Felt](seed),
			},
		},
		Nonces:            map[felt.Address]*felt.Felt{address: felt.NewFromUint64[felt.Felt](seed)},
		DeployedContracts: map[felt.Address]*felt.ClassHash{address: &classHash},
		DeclaredV0Classes: []*felt.ClassHash{&classHash},
		DeclaredV1Classes: map[felt.SierraClassHash]*felt.CasmClassHash{sierraClassHash: &casmClassHash},
		ReplacedClasses:   map[felt.Address]*felt.ClassHash{address: &classHash},
		MigratedClasses: map[felt.SierraClassHash]felt.CasmClassHash{
			sierraClassHash: casmClassHash,
		},
	}
}

func TestCloneCopiesEveryStateDiffField(t *testing.T) {
	original := fullStateDiff(1)
	// A new StateDiff field must be added to the fixture before the clone can miss it.
	fields := reflect.ValueOf(original)
	for i := range fields.NumField() {
		name := fields.Type().Field(i).Name
		require.False(t, fields.Field(i).IsZero(), "fixture must populate %s", name)
	}

	buildState := builder.BuildState{
		PreConfirmed: &pending.PreConfirmed{
			Block:       &core.Block{Header: &core.Header{}},
			StateUpdate: &core.StateUpdate{StateDiff: &original},
		},
	}
	cloned := buildState.Clone().PreConfirmed.StateUpdate.StateDiff
	require.Equal(t, fullStateDiff(1), *cloned)

	// Same address as original, so Merge writes into the inner storage map instead of adding an entry.
	extra := fullStateDiff(100)
	// Merge only appends to DeclaredV0Classes, which never rewrites a shared backing array.
	original.DeclaredV0Classes[0] = extra.DeclaredV0Classes[0]
	original.Merge(&extra)
	require.Equal(t, fullStateDiff(1), *cloned, "clone must not share storage with the original")
}
