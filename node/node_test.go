package node_test

import (
	"context"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	statetestutils "github.com/NethermindEth/juno/core/state/statetestutils"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/node"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
)

// Create a new node with all services enabled.
func TestNewNode(t *testing.T) {
	config := &node.Config{
		LogLevel:                           "info",
		HTTP:                               true,
		HTTPPort:                           0,
		Websocket:                          true,
		WebsocketPort:                      0,
		GRPC:                               true,
		GRPCPort:                           0,
		DatabasePath:                       t.TempDir(),
		Network:                            utils.Sepolia, // P2P will only work with Sepolia (for the time being)
		EthNode:                            "",
		DisableL1Verification:              true,
		Pprof:                              true,
		PprofPort:                          0,
		Colour:                             true,
		PendingPollInterval:                time.Second,
		PreConfirmedPollInterval:           time.Second,
		Metrics:                            true,
		MetricsPort:                        0,
		P2P:                                true,
		P2PAddr:                            "",
		P2PPeers:                           "",
		SubmittedTransactionsCacheEntryTTL: time.Second,
	}

	logLevel := utils.NewLogLevel(utils.INFO)
	n, err := node.New(config, "v0.3", logLevel)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	n.Run(ctx)
}

func TestNetworkVerificationOnNonEmptyDB(t *testing.T) {
	network := utils.Sepolia
	tests := map[string]struct {
		network   utils.Network
		errString string
	}{
		"same network": {
			network:   network,
			errString: "",
		},
		"different network": {
			network:   utils.Mainnet,
			errString: "unable to verify latest block hash; are the database and --network option compatible?",
		},
	}

	for description, test := range tests {
		t.Run(description, func(t *testing.T) {
			dbPath := t.TempDir()
			log := utils.NewNopZapLogger()
			database, err := pebble.New(dbPath)
			require.NoError(t, err)
			chain := blockchain.New(database, &network, statetestutils.UseNewState())
			ctx, cancel := context.WithCancel(t.Context())
			dataSource := sync.NewFeederGatewayDataSource(chain, adaptfeeder.New(feeder.NewTestClient(t, &network)))
			syncer := sync.New(chain, dataSource, log, 0, 0, false, database).
				WithListener(&sync.SelectiveListener{OnSyncStepDoneCb: func(op string, _ uint64, _ time.Duration) {
					// Stop the syncer after we successfully stored block.
					if op == sync.OpStore {
						cancel()
					}
				}})
			require.NoError(t, syncer.Run(ctx))
			cancel()
			require.NoError(t, database.Close())

			logLevel := utils.NewLogLevel(utils.INFO)
			_, err = node.New(&node.Config{
				DatabasePath:                       dbPath,
				Network:                            test.network,
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
