package core

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/NethermindEth/juno/starknet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	deprecatedFixturePath = "../clients/feeder/testdata/sepolia/class/0x5f18f9cdc05da87f04e8e7685bd346fc029f977167d5b1b2b59f69a7dacbfc8.json"
	// This is a byte-for-byte canonical-output snapshot, not a normal JSON fixture.
	// Keep the non-.json extension so editors do not autoformat it.
	canonicalProgramPath  = "testdata/deprecated_cairo_program.txt"
	upstreamArtifactsRoot = "testdata/starknet_rust/contracts/cairo0/artifacts"
)

type upstreamContractHashes struct {
	HintedClassHash string `json:"hinted_class_hash"`
}

type upstreamHashFixture struct {
	name     string
	artifact string
	hashes   string
}

// These fixtures and expected hashes are copied from the upstream starknet-core legacy.rs tests.
var upstreamHintedHashFixtures = []upstreamHashFixture{
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

func TestUpstreamHintedClassHashCorpus(t *testing.T) {
	for _, fixture := range upstreamHintedHashFixtures {
		t.Run(fixture.name, func(t *testing.T) {
			abi, program := loadDeprecatedFixtureProgram(t, fixture.artifact)
			want := loadUpstreamContractHashes(t, fixture.hashes)

			got, err := computeHintedClassHash(abi, program)
			require.NoError(t, err)
			assert.Equal(t, want.HintedClassHash, got.String())
		})
	}
}

func TestDeprecatedCairoProgramCanonicalSerialization(t *testing.T) {
	_, program := loadDeprecatedFixtureProgram(t, deprecatedFixturePath)

	var buffer bytes.Buffer
	require.NoError(t, writeDeprecatedCairoProgramCanonical(&buffer, program))

	want, err := os.ReadFile(filepath.Clean(canonicalProgramPath))
	require.NoError(t, err)
	assert.Equal(t, want, buffer.Bytes())
}

func loadUpstreamContractHashes(t *testing.T, path string) upstreamContractHashes {
	t.Helper()

	data, err := os.ReadFile(filepath.Clean(path))
	require.NoError(t, err)

	var hashes upstreamContractHashes
	require.NoError(t, json.Unmarshal(data, &hashes))
	return hashes
}

func loadDeprecatedFixtureProgram(t *testing.T, path string) (json.RawMessage, *deprecatedCairoProgram) {
	t.Helper()

	data, err := os.ReadFile(filepath.Clean(path))
	require.NoError(t, err)

	var definition starknet.ClassDefinition
	require.NoError(t, json.Unmarshal(data, &definition))
	require.NotNil(t, definition.DeprecatedCairo)

	program, err := unmarshalDeprecatedCairoProgram(definition.DeprecatedCairo.Program)
	require.NoError(t, err)

	return definition.DeprecatedCairo.Abi, program
}
