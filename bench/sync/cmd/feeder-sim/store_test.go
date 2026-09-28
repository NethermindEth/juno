package main

import (
	"os"
	"slices"
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
	rounds := preConfirmedFixtures(t)
	dataset := writeDataset(t, slices.Concat(all, rounds))

	tests := []struct {
		name         string
		preconfirmed bool
		want         []fixture
	}{
		{"blocks and classes", false, all},
		{"with pre-confirmed rounds", true, slices.Concat(all, rounds)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := fixtureConfig()
			config.preconfirmed = test.preconfirmed

			store, err := loadStore(t.Context(), dataset, config, log.NewNopZapLogger())
			require.NoError(t, err)
			require.Equal(t, fixtureBlocks, store.blocks)
			require.Len(t, store.bodies, len(test.want), "one body per dataset file, classes deduplicated")

			for _, fixture := range test.want {
				body, ok := store.get(fixture.resource.file)
				require.True(t, ok, fixture.resource.file)
				unzipped, err := gunzip(body)
				require.NoError(t, err)
				require.Equal(t, fixture.body, unzipped)
			}

			_, ok := store.get("get_block/1.json.gz")
			require.False(t, ok)
			_, ok = store.get(rounds[0].resource.file)
			require.Equal(t, test.preconfirmed, ok, "rounds are loaded only with --preconfirmed")
		})
	}
}

func TestLoadStoreMissingFile(t *testing.T) {
	all := slices.Concat(fixtures(t), preConfirmedFixtures(t))
	tests := []struct {
		name         string
		from, to     uint64
		preconfirmed bool
		missing      string
		wantHint     string
	}{
		{
			"contract addresses", fixtureFrom, fixtureTo, false,
			"get_contract_addresses.json.gz", "--network",
		},
		{"block", 56379, 56379, false, "get_block/56379.json.gz", "--network"},
		{"state update", 56379, 56379, false, "get_state_update/56379.json.gz", "--network"},
		{"class", 56377, 56377, false, "get_class_by_hash/" + deprecatedHash + ".json.gz", "--network"},
		{
			"compiled class", 56378, 56378, false,
			"get_compiled_class_by_class_hash/" + sierraHash + ".json.gz", "--network",
		},
		{
			"pre-confirmed round", 56379, 56379, true,
			"get_preconfirmed_block/56379.json.gz", "--network and --rpc-url",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dataset := writeDataset(t, all)
			require.NoError(t, os.Remove(dataset.path(test.missing)))

			config := testConfig(test.from, test.to, test.from)
			config.preconfirmed = test.preconfirmed
			_, err := loadStore(t.Context(), dataset, config, log.NewNopZapLogger())
			require.ErrorContains(t, err, test.missing+": missing; run with "+test.wantHint+" to capture")
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
