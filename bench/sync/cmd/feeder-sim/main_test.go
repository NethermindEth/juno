package main

import (
	"context"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/require"
)

func TestJunoFlags(t *testing.T) {
	tests := []struct {
		name    string
		network *networks.Network
		listen  string
		want    string
	}{
		{
			"sepolia on default listen", &networks.Sepolia, "",
			"--cn-name sepolia --cn-feeder-url http://127.0.0.1:7070/feeder_gateway/ " +
				"--cn-gateway-url http://127.0.0.1:7070/gateway/ --cn-l2-chain-id SN_SEPOLIA --cn-l1-chain-id 11155111 " +
				"--cn-core-contract-address 0xe2bb56ee936fd6433dc0f6e7e3b8365c906aa057 " +
				"--cn-unverifiable-range 0,0 --preconfirmed-poll-interval 0",
		},
		{
			"mainnet on all interfaces", &networks.Mainnet, ":9000",
			"--cn-name mainnet --cn-feeder-url http://:9000/feeder_gateway/ " +
				"--cn-gateway-url http://:9000/gateway/ --cn-l2-chain-id SN_MAIN --cn-l1-chain-id 1 " +
				"--cn-core-contract-address 0xc662c410c0ecf747543f5ba90660f6abebd9c8c4 " +
				"--cn-unverifiable-range 0,0 --preconfirmed-poll-interval 0",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, junoFlags(test.network, test.listen))
		})
	}
}

func TestCapture(t *testing.T) {
	all := fixtures(t)
	tests := []struct {
		name     string
		upstream []fixture
		network  func(upstream *upstream, t *testing.T) *networks.Network
		wantErr  string
	}{
		{"complete", all, (*upstream).network, ""},
		{
			"core contract mismatch", all,
			func(upstream *upstream, t *testing.T) *networks.Network {
				t.Helper()
				network := upstream.network(t)
				network.Name = networks.Mainnet.Name
				network.CoreContractAddress = networks.Mainnet.CoreContractAddress
				return network
			},
			"holds core contract 0xe2bb56ee936fd6433dc0f6e7e3b8365c906aa057, not --network mainnet's",
		},
		{
			"resource missing upstream", all[:len(all)-1], (*upstream).network,
			all[len(all)-1].resource.file + ": unexpected status 400",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			upstream := newUpstream(t, test.upstream)
			dataset := dataset{root: t.TempDir()}
			config := fixtureConfig()
			config.network = test.network(upstream, t)

			err := capture(t.Context(), dataset, config, log.NewNopZapLogger())
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, int64(len(all)), upstream.requests.Load())
			for _, fixture := range all {
				stored, err := dataset.read(fixture.resource.file)
				require.NoError(t, err)
				unzipped, err := gunzip(stored)
				require.NoError(t, err)
				require.Equal(t, fixture.body, unzipped)
			}

			require.NoError(t, capture(t.Context(), dataset, config, log.NewNopZapLogger()))
			require.Equal(
				t, int64(len(all)), upstream.requests.Load(),
				"a complete dataset is not fetched again",
			)
		})
	}
}

func TestCaptureResumesPartialDataset(t *testing.T) {
	all := fixtures(t)
	upstream := newUpstream(t, all)
	dataset := writeDataset(t, all[:4])
	config := fixtureConfig()
	config.network = upstream.network(t)

	require.NoError(t, capture(t.Context(), dataset, config, log.NewNopZapLogger()))
	require.Equal(t, int64(len(all)-4), upstream.requests.Load())
}

func TestRunCaptureOnly(t *testing.T) {
	all := fixtures(t)
	upstream := newUpstream(t, all)
	config := fixtureConfig()
	config.data = t.TempDir()
	config.network = upstream.network(t)
	config.logLevel = log.NewLevel(log.ERROR)

	require.NoError(t, run(t.Context(), config))
	require.Equal(t, int64(len(all)), upstream.requests.Load())

	store, err := loadStore(t.Context(), dataset{root: config.data}, config, log.NewNopZapLogger())
	require.NoError(t, err)
	require.Equal(t, fixtureBlocks, store.blocks)
}

func TestServeStopsOnCancel(t *testing.T) {
	config := fixtureConfig()
	config.listen = "127.0.0.1:0"
	config.interval = time.Hour
	ctx, cancel := context.WithCancel(t.Context())

	dataset := fixtureDataset(t)

	done := make(chan error, 1)
	go func() { done <- serve(ctx, dataset, config, log.NewNopZapLogger()) }()
	time.AfterFunc(200*time.Millisecond, cancel)

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("serve did not stop after cancellation")
	}
}
