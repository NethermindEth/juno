package node_test

import (
	"context"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/clients/feeder"
	statetestutils "github.com/NethermindEth/juno/core/state/testutils"
	"github.com/NethermindEth/juno/db/pebblev2"
	"github.com/NethermindEth/juno/node"
	"github.com/NethermindEth/juno/starknet/compiler"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/require"
)

// Create a new node with all services enabled.
func TestNewNode(t *testing.T) {
	config := &node.Config{
		LogLevel:      "info",
		HTTP:          true,
		HTTPPort:      0,
		Websocket:     true,
		WebsocketPort: 0,
		GRPC:          true,
		GRPCPort:      0,
		DatabasePath:  t.TempDir(),
		DBCompression: "zstd",
		// P2P will only work with Sepolia (for the time being)
		Network:                            networks.Sepolia,
		EthNode:                            "",
		DisableL1Verification:              true,
		Pprof:                              true,
		PprofPort:                          0,
		Colour:                             true,
		PreConfirmedPollInterval:           time.Second,
		Metrics:                            true,
		MetricsPort:                        0,
		P2P:                                true,
		P2PAddr:                            "",
		P2PPeers:                           "",
		SubmittedTransactionsCacheEntryTTL: time.Second,
		// MaxConcurrentCompilations left unset (not Explicit) to exercise the auto-derive path.
	}

	logLevel := log.NewLevel(log.INFO)
	n, err := node.New(config, "v0.3", logLevel)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	n.Run(ctx)
}

func TestNewNodeWithSyncDisabled(t *testing.T) {
	config := &node.Config{
		LogLevel:                           "info",
		HTTP:                               true,
		HTTPPort:                           0,
		DatabasePath:                       t.TempDir(),
		DBCompression:                      "zstd",
		Network:                            networks.Sepolia,
		DisableL1Verification:              true,
		DisableSync:                        true,
		SubmittedTransactionsCacheEntryTTL: time.Second,
	}

	n, err := node.New(config, "v0.3", log.NewLevel(log.INFO))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	n.Run(ctx)
}

func TestNewNodeRunsOneAtATimeOnLowMemory(t *testing.T) {
	config := &node.Config{
		LogLevel:                           "info",
		HTTP:                               true,
		DatabasePath:                       t.TempDir(),
		DBCompression:                      "zstd",
		Network:                            networks.Sepolia,
		DisableL1Verification:              true,
		SubmittedTransactionsCacheEntryTTL: time.Second,
		// MaxConcurrentCompilations left unset: derive, then floor to 1.
		MaxCompilationMemory: 4096,
		// Reserve more than the machine has, so nothing fits.
		NodeMemoryReserve: uint(compiler.AvailableMemoryMB() + 4096),
	}

	_, err := node.New(config, "v0.3", log.NewLevel(log.INFO))
	require.NoError(t, err)
}

func TestNewNodeSkipsDerivedConcurrency(t *testing.T) {
	config := &node.Config{
		LogLevel:                           "info",
		HTTP:                               true,
		DatabasePath:                       t.TempDir(),
		DBCompression:                      "zstd",
		Network:                            networks.Sepolia,
		DisableL1Verification:              true,
		SubmittedTransactionsCacheEntryTTL: time.Second,
		MaxConcurrentCompilations:          2,
		MaxConcurrentCompilationsExplicit:  true,
	}

	_, err := node.New(config, "v0.3", log.NewLevel(log.INFO))
	require.NoError(t, err)
}

func TestNewNodeSkipsDerivedQueue(t *testing.T) {
	config := &node.Config{
		LogLevel:                           "info",
		HTTP:                               true,
		DatabasePath:                       t.TempDir(),
		DBCompression:                      "zstd",
		Network:                            networks.Sepolia,
		DisableL1Verification:              true,
		SubmittedTransactionsCacheEntryTTL: time.Second,
		MaxCompilationQueue:                8,
		MaxCompilationQueueExplicit:        true,
	}

	_, err := node.New(config, "v0.3", log.NewLevel(log.INFO))
	require.NoError(t, err)
}

func TestNetworkVerificationOnNonEmptyDB(t *testing.T) {
	network := networks.Sepolia
	tests := map[string]struct {
		network   networks.Network
		errString string
	}{
		"same network": {
			network:   network,
			errString: "",
		},
		"different network": {
			network:   networks.Mainnet,
			errString: "unable to verify latest block hash; are the database and --network option compatible?",
		},
	}

	for description, test := range tests {
		t.Run(description, func(t *testing.T) {
			dbPath := t.TempDir()
			logger := log.NewNopZapLogger()
			database, err := pebblev2.New(dbPath)
			require.NoError(t, err)
			chain := blockchain.New(
				database,
				&network,
				blockchain.WithNewState(statetestutils.UseNewState()),
			)
			ctx, cancel := context.WithCancel(t.Context())
			dataSource := sync.NewFeederGatewayDataSource(chain, adaptfeeder.New(feeder.NewTestClient(t, &network)))
			syncer := sync.New(chain, dataSource, logger, 0, false, database).
				WithListener(&sync.SelectiveListener{OnSyncStepDoneCb: func(op string, _ uint64, _ time.Duration) {
					// Stop the syncer after we successfully stored block.
					if op == sync.OpStore {
						cancel()
					}
				}})
			require.NoError(t, syncer.Run(ctx))
			cancel()
			require.NoError(t, database.Close())

			logLevel := log.NewLevel(log.INFO)
			_, err = node.New(&node.Config{
				DatabasePath:                       dbPath,
				DBCompression:                      "zstd",
				Network:                            test.network,
				NewState:                           statetestutils.UseNewState(),
				DisableL1Verification:              true,
				SubmittedTransactionsCacheEntryTTL: time.Second,
			}, "v0.1", logLevel)
			if test.errString == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, test.errString)
			}
		})
	}
}

func TestNew_RejectsPruneWithoutL1Verification(t *testing.T) {
	_, err := node.New(&node.Config{
		Prune:                 true,
		DisableL1Verification: true,
	}, "test", log.NewLevel(log.INFO))
	require.ErrorContains(t, err, "prune-mode requires L1 verification")
}

func TestNewRejectsModesThatRequireSynchronization(t *testing.T) {
	tests := map[string]struct {
		config    node.Config
		errString string
	}{
		"sequencer": {
			config:    node.Config{Sequencer: true},
			errString: "disable-sync has no effect in sequencer mode",
		},
		"p2p": {
			config:    node.Config{P2P: true},
			errString: "p2p requires synchronization",
		},
		"pruning": {
			config:    node.Config{Prune: true},
			errString: "prune-mode requires synchronization",
		},
		"remote database": {
			config:    node.Config{RemoteDB: "localhost:6064"},
			errString: "remote-db cannot be combined with --disable-sync",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			test.config.DisableSync = true
			_, err := node.New(&test.config, "test", log.NewLevel(log.INFO))
			require.ErrorContains(t, err, test.errString)
		})
	}
}
