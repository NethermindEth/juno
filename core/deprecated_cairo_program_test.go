package core_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const upstreamArtifactsRoot = "testdata/starknet_rust/contracts/cairo0/artifacts"

type upstreamContractHashes struct {
	ClassHash string `json:"class_hash"`
}

type upstreamHashFixture struct {
	name     string
	artifact string
	hashes   string
}

// Fixtures and expected hashes are copied from upstream starknet-core tests:
// https://github.com/software-mansion/starknet-rust/tree/master/starknet-rust-core/test-data/contracts/cairo0/artifacts
var upstreamCairo0HashFixtures = []upstreamHashFixture{
	{
		name:     "OZ account",
		artifact: filepath.Join(upstreamArtifactsRoot, "oz_account.txt"),
		hashes:   filepath.Join(upstreamArtifactsRoot, "oz_account.hashes.json"),
	},
	{
		name:     "Emoji",
		artifact: filepath.Join(upstreamArtifactsRoot, "emoji.txt"),
		hashes:   filepath.Join(upstreamArtifactsRoot, "emoji.hashes.json"),
	},
	{
		name:     "Pre 0.11 OZ account",
		artifact: filepath.Join(upstreamArtifactsRoot, "pre-0.11.0", "oz_account.txt"),
		hashes:   filepath.Join(upstreamArtifactsRoot, "pre-0.11.0", "oz_account.hashes.json"),
	},
	{
		name:     "Pre 0.10 Braavos proxy",
		artifact: filepath.Join(upstreamArtifactsRoot, "pre-0.10.0", "braavos_proxy.txt"),
		hashes:   filepath.Join(upstreamArtifactsRoot, "pre-0.10.0", "braavos_proxy.hashes.json"),
	},
}

func TestUpstreamCairo0ClassHashCorpus(t *testing.T) {
	for _, fixture := range upstreamCairo0HashFixtures {
		t.Run(fixture.name, func(t *testing.T) {
			class := loadUpstreamDeprecatedClass(t, fixture.artifact)
			want := loadUpstreamContractHashes(t, fixture.hashes)

			got, err := class.Hash()
			require.NoError(t, err)
			assert.Equal(t, felt.UnsafeFromString[felt.Felt](want.ClassHash), got)
		})
	}
}

func loadUpstreamDeprecatedClass(t *testing.T, path string) *core.DeprecatedCairoClass {
	t.Helper()

	data, err := os.ReadFile(filepath.Clean(path))
	require.NoError(t, err)

	var definition starknet.ClassDefinition
	require.NoError(t, json.Unmarshal(data, &definition))
	require.NotNil(t, definition.DeprecatedCairo)

	classDef, err := sn2core.AdaptDeprecatedCairoClass(definition.DeprecatedCairo)
	require.NoError(t, err)

	class, ok := classDef.(*core.DeprecatedCairoClass)
	require.True(t, ok)
	return class
}

func loadUpstreamContractHashes(t *testing.T, path string) upstreamContractHashes {
	t.Helper()

	data, err := os.ReadFile(filepath.Clean(path))
	require.NoError(t, err)

	var hashes upstreamContractHashes
	require.NoError(t, json.Unmarshal(data, &hashes))
	return hashes
}
