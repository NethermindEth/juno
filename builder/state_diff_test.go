package builder_test

import (
	"testing"

	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/require"
)

var (
	newContractAddr      = new(felt.Felt).SetUint64(100)
	newClassHash         = new(felt.Felt).SetUint64(101)
	newCompiledClassHash = new(felt.Felt).SetUint64(102)
	key                  = new(felt.Felt)
	value                = new(felt.Felt).SetUint64(1)

	builderDiff = builder.StateDiff{
		StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
			*newContractAddr: {*key: value},
		},
		Nonces: map[felt.Felt]*felt.Felt{
			*contractAddr: new(felt.Felt).SetUint64(3),
		},
		DeployedContracts: map[felt.Felt]*felt.Felt{*newContractAddr: newClassHash},
		DeclaredV0Classes: []*felt.Felt{},
		DeclaredV1Classes: map[felt.Felt]*felt.Felt{*newClassHash: newCompiledClassHash},
		ReplacedClasses:   map[felt.Felt]*felt.Felt{*contractAddr: newClassHash},
		Classes:           map[felt.Felt]core.Class{*newClassHash: testClass},
	}

	coreDiff = core.StateDiff{
		StorageDiffs: map[felt.Felt][]core.StorageDiff{
			*newContractAddr: {{Key: key, Value: value}},
		},
		Nonces:            map[felt.Felt]*felt.Felt{*contractAddr: new(felt.Felt).SetUint64(3)},
		DeployedContracts: []core.AddressClassHashPair{{Address: newContractAddr, ClassHash: newClassHash}},
		DeclaredV0Classes: []*felt.Felt{},
		DeclaredV1Classes: []core.DeclaredV1Class{{ClassHash: newClassHash, CompiledClassHash: newCompiledClassHash}},
		ReplacedClasses:   []core.AddressClassHashPair{{Address: contractAddr, ClassHash: newClassHash}},
	}
	coreClasses = map[felt.Felt]core.Class{*newClassHash: testClass}
)

func TestAdapters(t *testing.T) {
	gotCoreDiff, classes := builder.AdaptStateDiffToCoreDiff(&builderDiff)
	require.Equal(t, &coreDiff, gotCoreDiff)
	require.Equal(t, coreClasses, classes)

	gotBuilderDiff := builder.AdaptCoreDiffToExecutionDiff(&coreDiff, coreClasses)
	require.Equal(t, &builderDiff, gotBuilderDiff)
}
