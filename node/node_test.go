package node_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/NethermindEth/juno/node"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultDbPath(t *testing.T) {
	defaultDataDir, err := utils.DefaultDataDir()
	require.NoError(t, err)

	networks := []utils.Network{utils.GOERLI, utils.MAINNET, utils.GOERLI2, utils.INTEGRATION}

	for _, n := range networks {
		t.Run(n.String(), func(t *testing.T) {
			cfg := &node.Config{Network: n, DatabasePath: ""}
			expectedCfg := node.Config{
				Network:      n,
				DatabasePath: filepath.Join(defaultDataDir, n.String()),
			}
			snNode, err := node.New(cfg, "1.2.3")
			require.NoError(t, err)

			assert.Equal(t, expectedCfg, snNode.Config())
		})
	}
}

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
		Network:             utils.MAINNET,
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

	_, err := node.New(config, "v0.3")
	require.NoError(t, err)
}
