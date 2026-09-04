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
	address := felt.NewFromUint64[felt.Felt](1)
	classHash := felt.NewFromUint64[felt.Felt](seed)
	return core.StateDiff{
		StorageDiffs:      map[felt.Felt]map[felt.Felt]*felt.Felt{*address: {*classHash: classHash}},
		Nonces:            map[felt.Felt]*felt.Felt{*address: classHash},
		DeployedContracts: map[felt.Felt]*felt.Felt{*address: classHash},
		DeclaredV0Classes: []*felt.Felt{classHash},
		DeclaredV1Classes: map[felt.Felt]*felt.Felt{*classHash: classHash},
		ReplacedClasses:   map[felt.Felt]*felt.Felt{*address: classHash},
		MigratedClasses: map[felt.SierraClassHash]felt.CasmClassHash{
			felt.SierraClassHash(*classHash): felt.CasmClassHash(*classHash),
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
