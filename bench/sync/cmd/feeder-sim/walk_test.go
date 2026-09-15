package main

import (
	"context"
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/require"
)

func TestWalkCoreContract(t *testing.T) {
	tests := []struct {
		name    string
		network *networks.Network
		wantErr string
	}{
		{"offline skips the check", nil, ""},
		{"matching network", &networks.Sepolia, ""},
		{
			"other network",
			&networks.Mainnet,
			"holds core contract 0xe2bb56ee936fd6433dc0f6e7e3b8365c906aa057, " +
				"not --network mainnet's 0xc662c410c0ecf747543f5ba90660f6abebd9c8c4",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dataset := fixtureDataset(t)
			config := fixtureConfig()
			config.network = test.network

			_, err := loadStore(t.Context(), dataset, config, log.NewNopZapLogger())
			if test.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, dataset.root+" "+test.wantErr)
		})
	}
}

func TestWalkRejectsCorruptFiles(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		body    []byte
		wantErr string
	}{
		{
			"contract addresses",
			"get_contract_addresses.json.gz",
			[]byte(contractAddressesJSON),
			"get_contract_addresses: gzip: invalid header",
		},
		{
			"state update",
			"get_state_update/56379.json.gz",
			gzipped(t, []byte("{}")),
			"get_state_update 56379: state update response lacks block or state_update",
		},
		{
			"class",
			"get_class_by_hash/" + sierraHash + ".json.gz",
			gzipped(t, []byte("[]")),
			"get_class_by_hash " + sierraHash + ":",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dataset := fixtureDataset(t)
			require.NoError(t, dataset.write(test.file, test.body))
			config := fixtureConfig()
			config.network = &networks.Sepolia

			_, err := loadStore(t.Context(), dataset, config, log.NewNopZapLogger())
			require.ErrorContains(t, err, test.wantErr)
		})
	}
}

// recorder is a fetcher that records every resource the walker asks for.
type recorder struct {
	visited []string
}

func (recorder *recorder) fetch(
	_ context.Context,
	dataset dataset,
	resource resource,
) ([]byte, error) {
	recorder.visited = append(recorder.visited, resource.file)
	return dataset.read(resource.file)
}

func TestWalkVisitsEachResourceOnce(t *testing.T) {
	all := fixtures(t)
	recorder := &recorder{}
	walker := &walker{
		fetcher:     recorder,
		dataset:     writeDataset(t, all),
		config:      fixtureConfig(),
		concurrency: 1,
		logger:      log.NewNopZapLogger(),
	}

	blocks, err := walker.walk(t.Context())
	require.NoError(t, err)
	require.Equal(t, fixtureBlocks, blocks)

	want := make([]string, 0, len(all))
	for _, fixture := range all {
		want = append(want, fixture.resource.file)
	}
	require.ElementsMatch(t, want, recorder.visited)
}
