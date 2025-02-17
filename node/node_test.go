package node_test

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/node"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

// Create a new node with all services enabled.
func TestNewNode(t *testing.T) {
	config := &node.Config{
		LogLevel:              "info",
		HTTP:                  true,
		HTTPPort:              0,
		Websocket:             true,
		WebsocketPort:         0,
		GRPC:                  true,
		GRPCPort:              0,
		DatabasePath:          t.TempDir(),
		Network:               utils.Sepolia, // P2P will only work with Sepolia (for the time being)
		EthNode:               "",
		DisableL1Verification: true,
		Pprof:                 true,
		PprofPort:             0,
		Colour:                true,
		PendingPollInterval:   time.Second,
		Metrics:               true,
		MetricsPort:           0,
		P2P:                   true,
		P2PAddr:               "",
		P2PPeers:              "",
	}

	logLevel := utils.NewLogLevel(utils.INFO)
	n, err := node.New(config, "v0.3", logLevel)
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
			chain := blockchain.New(database, &network, nil)
			syncer := sync.New(chain, adaptfeeder.New(feeder.NewTestClient(t, &network)), log, 0, false, database)
			ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
			require.NoError(t, syncer.Run(ctx))
			cancel()
			require.NoError(t, database.Close())

			logLevel := utils.NewLogLevel(utils.INFO)
			_, err = node.New(&node.Config{
				DatabasePath:          dbPath,
				Network:               test.network,
				DisableL1Verification: true,
			}, "v0.1", logLevel)
			if test.errString == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, test.errString)
			}
		})
	}
}

func TestNodeWithL1Verification(t *testing.T) {
	server := rpc.NewServer()
	require.NoError(t, server.RegisterName("eth", &testEmptyService{}))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() {
		_ = http.Serve(listener, server.WebsocketHandler([]string{"*"}))
	}()
	defer server.Stop()
	_, err = node.New(&node.Config{
		Network:               utils.Network{},
		EthNode:               "ws://" + listener.Addr().String(),
		DatabasePath:          t.TempDir(),
		DisableL1Verification: false,
	}, "v0.1", utils.NewLogLevel(utils.INFO))
	require.NoError(t, err)
}

func TestNodeWithL1VerificationError(t *testing.T) {
	tests := []struct {
		name string
		cfg  *node.Config
		err  string
	}{
		{
			name: "no network",
			cfg: &node.Config{
				DatabasePath:          t.TempDir(),
				DisableL1Verification: false,
			},
			err: "ethereum node address not found",
		},
		{
			name: "parce URL error",
			cfg: &node.Config{
				DatabasePath:          t.TempDir(),
				DisableL1Verification: false,
				EthNode:               string([]byte{0x7f}),
			},
			err: "parse Ethereum node URL",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := node.New(tt.cfg, "v0.1", utils.NewLogLevel(utils.INFO))
			require.ErrorContains(t, err, tt.err)
		})
	}
}

type testEmptyService struct{}

func (testEmptyService) GetBlockByNumber(ctx context.Context, number string, fullTx bool) (interface{}, error) {
	return nil, nil
}
