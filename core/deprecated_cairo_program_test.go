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

const (
	upstreamArtifactsRoot   = "testdata/starknet_rust/contracts/cairo0/artifacts"
	regressionArtifactsRoot = "testdata/cairo0_hash_regressions"
)

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

var cairo0HashRegressionFixtures = []upstreamHashFixture{
	{
		name: "Real-world pre-0.10 cairo_type spacing quirk",
		artifact: filepath.Join(
			regressionArtifactsRoot,
			"0xa0cb53aaa37a4d377736e7e98c1a96b5168d75e3705f30fb09e6d2cbd7d5e3.txt",
		),
		hashes: filepath.Join(
			regressionArtifactsRoot,
			"0xa0cb53aaa37a4d377736e7e98c1a96b5168d75e3705f30fb09e6d2cbd7d5e3.hashes.json",
		),
	},
	{
		name: "Null ABI legacy class",
		artifact: filepath.Join(
			regressionArtifactsRoot,
			"0x371b5f7c5517d84205365a87f02dcef230efa7b4dd91a9e4ba7e04c5b69d69b.txt",
		),
		hashes: filepath.Join(
			regressionArtifactsRoot,
			"0x371b5f7c5517d84205365a87f02dcef230efa7b4dd91a9e4ba7e04c5b69d69b.hashes.json",
		),
	},
	{
		name: "Pre-0.10 reference cairo_type spacing quirk",
		artifact: filepath.Join(
			regressionArtifactsRoot,
			"0x6dc10e7703c1b63e0b5a4e8e7842293d3255fd4e53d4e730adf435c3dffabb.txt",
		),
		hashes: filepath.Join(
			regressionArtifactsRoot,
			"0x6dc10e7703c1b63e0b5a4e8e7842293d3255fd4e53d4e730adf435c3dffabb.hashes.json",
		),
	},
}

func TestCairo0ClassHashRegressions(t *testing.T) {
	for _, fixture := range cairo0HashRegressionFixtures {
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
