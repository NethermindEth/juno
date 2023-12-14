package node_test

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/node"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/common"
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
		Network:             utils.Mainnet,
		EthNode:             "",
		Pprof:               true,
		PprofPort:           0,
		Colour:              true,
		PendingPollInterval: time.Second,
		Metrics:             true,
		MetricsPort:         0,
		P2P:                 true,
		P2PAddr:             "",
		P2PBootPeers:        "",
	}

	n, err := node.New(config, "v0.3")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	n.Run(ctx)
}

// Create a new node with all services enabled, except for Metrics,
// because it panics with "duplicate metrics collector registration attempted"
func TestNewCustomNode(t *testing.T) {
	config := &node.Config{
		LogLevel:      utils.INFO,
		HTTP:          true,
		HTTPPort:      0,
		Websocket:     true,
		WebsocketPort: 0,
		GRPC:          true,
		GRPCPort:      0,
		DatabasePath:  t.TempDir(),
		Network:       utils.Custom,
		NetworkCustom: utils.NetworkCustom{
			FeederURLVal:  "some_url",
			GatewayURLVal: "some_other_url",
			ChainIDVal:    "SN_CUSTOM",
			L1ChainIDVal:  big.NewInt(5),
			ProtocolIDVal: 2,
		},
		EthNode:             "",
		Pprof:               true,
		PprofPort:           0,
		Colour:              true,
		PendingPollInterval: time.Second,
		Metrics:             false,
		MetricsPort:         0,
		P2P:                 true,
		P2PAddr:             "",
		P2PBootPeers:        "",
	}

	commonAddressSuccess := common.HexToAddress("0x0")

	tests := map[string]struct {
		commonAddress common.Address
		err           error
	}{
		"success": {
			commonAddress: commonAddressSuccess,
			err:           nil,
		},
	}

	for description, test := range tests {
		t.Run(description, func(t *testing.T) {
			testcommonAddress := test.commonAddress
			config.NetworkCustom.CoreContractAddressVal = &testcommonAddress
			n, err := node.New(config, "v0.3")
			if test.err != nil {
				require.Equal(t, test.err, err)
			} else {
				require.Equal(t, "custom", n.Config().Network.String())
				ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
				n.Run(ctx)
				cancel()
			}
		})
	}
}

func TestGetNetwork(t *testing.T) {
	t.Run("custom network", func(t *testing.T) {
		commonAddressSuccess := common.HexToAddress("0x1")
		config := &node.Config{
			Network: utils.Custom,
			NetworkCustom: utils.NetworkCustom{
				FeederURLVal:           "some_url",
				GatewayURLVal:          "some_other_url",
				ChainIDVal:             "SN_CUSTOM",
				L1ChainIDVal:           big.NewInt(5),
				ProtocolIDVal:          2,
				CoreContractAddressVal: &commonAddressSuccess,
			},
		}
		network, err := config.GetNetwork()
		require.NoError(t, err)
		require.Equal(t, config.NetworkCustom, network)
	})

	t.Run("known network", func(t *testing.T) {
		config := &node.Config{
			Network: utils.Mainnet,
		}

		network, err := config.GetNetwork()
		require.NoError(t, err)
		require.Equal(t, config.Network, network)
	})
}

func TestNetworkVerificationOnNonEmptyDB(t *testing.T) {
	network := utils.Integration
	tests := map[string]struct {
		network   utils.NetworkKnown
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
			database, err := pebble.New(dbPath, 1, log)
			require.NoError(t, err)
			chain := blockchain.New(database, network, log)
			syncer := sync.New(chain, adaptfeeder.New(feeder.NewTestClient(t, network)), log, 0, false)
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
