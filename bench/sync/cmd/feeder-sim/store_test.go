package main

import (
	"os"
	"testing"

	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/require"
)

var fixtureBlocks = []blockInfo{
	{number: 56377, timestamp: 1712213818, classHashes: []string{deprecatedHash}},
	{number: 56378, timestamp: 1712214058, classHashes: []string{sierraHash, sierraHash}},
	{number: 56379, timestamp: 1712214299, classHashes: []string{}},
}

func TestLoadStore(t *testing.T) {
	all := fixtures(t)
	dataset := writeDataset(t, all)

	store, err := loadStore(t.Context(), dataset, fixtureConfig(), log.NewNopZapLogger())
	require.NoError(t, err)
	require.Equal(t, fixtureBlocks, store.blocks)
	require.Len(t, store.bodies, len(all), "one body per dataset file, classes deduplicated")

	for _, fixture := range all {
		body, ok := store.get(fixture.resource.file)
		require.True(t, ok, fixture.resource.file)
		unzipped, err := gunzip(body)
		require.NoError(t, err)
		require.Equal(t, fixture.body, unzipped)
	}

	_, ok := store.get("get_block/1.json.gz")
	require.False(t, ok)
}

func TestLoadStoreMissingFile(t *testing.T) {
	tests := []struct {
		name     string
		from, to uint64
		missing  string
	}{
		{"contract addresses", fixtureFrom, fixtureTo, "get_contract_addresses.json.gz"},
		{"block", 56379, 56379, "get_block/56379.json.gz"},
		{"state update", 56379, 56379, "get_state_update/56379.json.gz"},
		{"class", 56377, 56377, "get_class_by_hash/" + deprecatedHash + ".json.gz"},
		{"compiled class", 56378, 56378, "get_compiled_class_by_class_hash/" + sierraHash + ".json.gz"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dataset := fixtureDataset(t)
			require.NoError(t, os.Remove(dataset.path(test.missing)))

			config := testConfig(test.from, test.to, test.from)
			_, err := loadStore(t.Context(), dataset, config, log.NewNopZapLogger())
			require.ErrorContains(t, err, test.missing+": missing; run with --network to capture")
		})
	}
}

func TestLoadStoreSubrange(t *testing.T) {
	config := testConfig(56378, 56378, 56378)
	store, err := loadStore(t.Context(), fixtureDataset(t), config, log.NewNopZapLogger())
	require.NoError(t, err)
	require.Equal(t, fixtureBlocks[1:2], store.blocks)

	// contract addresses, block, state update, sierra class and its compiled class
	require.Len(t, store.bodies, 5)
	_, ok := store.get("get_class_by_hash/" + deprecatedHash + ".json.gz")
	require.False(t, ok, "classes outside the range are not loaded")
}
