package node_test

import (
	"context"
	"testing"
	"time"

	"github.com/NethermindEth/juno/node"
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
