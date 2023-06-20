package node_test

import (
	"path/filepath"
	"testing"

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
