package node_test

import (
	"context"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
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
		LogLevel:            utils.INFO,
		HTTP:                true,
		HTTPPort:            0,
		Websocket:           true,
		WebsocketPort:       0,
		GRPC:                true,
		GRPCPort:            0,
		DatabasePath:        t.TempDir(),
		Network:             utils.Sepolia, // P2P will only work with Sepolia (for the time being)
		EthNode:             "",
		Pprof:               true,
		PprofPort:           0,
		Colour:              true,
		PendingPollInterval: time.Second,
		Metrics:             true,
		MetricsPort:         0,
		P2P:                 true,
		P2PAddr:             "",
		P2PPeers:            "",
	}

	n, err := node.New(config, "v0.3")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	n.Run(ctx)
}

func TestNetworkVerificationOnNonEmptyDB(t *testing.T) {
	network := utils.Integration
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
			chain := blockchain.New(database, &network)
			syncer := sync.New(chain, adaptfeeder.New(feeder.NewTestClient(t, &network)), log, 0, false)
			ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
			require.NoError(t, syncer.Run(ctx))
			cancel()
			require.NoError(t, database.Close())

			_, err = node.New(&node.Config{
				DatabasePath: dbPath,
				Network:      test.network,
			}, "v0.1")
			if test.errString == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, test.errString)
			}
		})
	}
}
