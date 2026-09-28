package main

import (
	"context"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/require"
)

func TestJunoFlags(t *testing.T) {
	tests := []struct {
		name         string
		network      *networks.Network
		listen       string
		preconfirmed bool
		want         string
	}{
		{
			"sepolia on default listen", &networks.Sepolia, "", false,
			"--cn-name sepolia --cn-feeder-url http://127.0.0.1:7070/feeder_gateway/ " +
				"--cn-gateway-url http://127.0.0.1:7070/gateway/ --cn-l2-chain-id SN_SEPOLIA --cn-l1-chain-id 11155111 " +
				"--cn-core-contract-address 0xe2bb56ee936fd6433dc0f6e7e3b8365c906aa057 " +
				"--cn-unverifiable-range 0,0 --preconfirmed-poll-interval 0",
		},
		{
			"mainnet on all interfaces", &networks.Mainnet, ":9000", false,
			"--cn-name mainnet --cn-feeder-url http://:9000/feeder_gateway/ " +
				"--cn-gateway-url http://:9000/gateway/ --cn-l2-chain-id SN_MAIN --cn-l1-chain-id 1 " +
				"--cn-core-contract-address 0xc662c410c0ecf747543f5ba90660f6abebd9c8c4 " +
				"--cn-unverifiable-range 0,0 --preconfirmed-poll-interval 0",
		},
		{
			"sepolia with pre-confirmed", &networks.Sepolia, "", true,
			"--cn-name sepolia --cn-feeder-url http://127.0.0.1:7070/feeder_gateway/ " +
				"--cn-gateway-url http://127.0.0.1:7070/gateway/ --cn-l2-chain-id SN_SEPOLIA --cn-l1-chain-id 11155111 " +
				"--cn-core-contract-address 0xe2bb56ee936fd6433dc0f6e7e3b8365c906aa057 " +
				"--cn-unverifiable-range 0,0",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, junoFlags(test.network, test.listen, test.preconfirmed))
		})
	}
}

func TestCapture(t *testing.T) {
	all := fixtures(t)
	rounds := preConfirmedFixtures(t)
	traces := fixtureTraces(t)
	incompleteTraces := maps.Clone(traces)
	delete(incompleteTraces, fixtureTo)

	tests := []struct {
		name         string
		upstream     []fixture
		traces       map[uint64][]byte
		preconfirmed bool
		network      func(upstream *upstream, t *testing.T) *networks.Network
		wantStored   []fixture
		wantErr      string
	}{
		{"complete", all, nil, false, (*upstream).network, all, ""},
		{
			"core contract mismatch", all, nil, false,
			func(upstream *upstream, t *testing.T) *networks.Network {
				t.Helper()
				network := upstream.network(t)
				network.Name = networks.Mainnet.Name
				network.CoreContractAddress = networks.Mainnet.CoreContractAddress
				return network
			},
			nil, "holds core contract 0xe2bb56ee936fd6433dc0f6e7e3b8365c906aa057, not --network mainnet's",
		},
		{
			"resource missing upstream", all[:len(all)-1], nil, false, (*upstream).network,
			nil, all[len(all)-1].resource.file + ": unexpected status 400",
		},
		{
			"with pre-confirmed rounds", all, traces, true, (*upstream).network,
			slices.Concat(all, rounds), "",
		},
		{
			"trace missing upstream", all, incompleteTraces, true, (*upstream).network,
			nil, "get_preconfirmed_block/56379.json.gz: tracing block 56379: rpc error 24: Block not found",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			upstream := newUpstream(t, test.upstream)
			rpc := newRPCUpstream(t, test.traces)
			dataset := dataset{root: t.TempDir()}
			config := fixtureConfig()
			config.network = test.network(upstream, t)
			config.preconfirmed = test.preconfirmed
			config.rpc = rpc.url(t)

			err := capture(t.Context(), dataset, config, log.NewNopZapLogger())
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)

			wantRPCRequests := int64(len(test.wantStored) - len(all))
			require.Equal(t, int64(len(all)), upstream.requests.Load())
			require.Equal(t, wantRPCRequests, rpc.requests.Load())
			for _, fixture := range test.wantStored {
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
			require.Equal(t, wantRPCRequests, rpc.requests.Load(), "a complete dataset is not traced again")
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
	dataset := writeDataset(t, slices.Concat(fixtures(t), preConfirmedFixtures(t)))
	tests := []struct {
		name   string
		config *config
	}{
		{"feeder only", fixtureConfig()},
		{"with pre-confirmed", preConfirmedConfig(fixtureFrom)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := test.config
			config.listen = "127.0.0.1:0"
			config.interval = time.Hour
			ctx, cancel := context.WithCancel(t.Context())

			done := make(chan error, 1)
			go func() { done <- serve(ctx, dataset, config, log.NewNopZapLogger()) }()
			time.AfterFunc(200*time.Millisecond, cancel)

			select {
			case err := <-done:
				require.NoError(t, err)
			case <-time.After(10 * time.Second):
				t.Fatal("serve did not stop after cancellation")
			}
		})
	}
}
