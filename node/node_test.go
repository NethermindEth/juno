package node_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/NethermindEth/juno/node"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("network", func(t *testing.T) {
		networks := []utils.Network{
			utils.GOERLI, utils.MAINNET, utils.GOERLI2, utils.INTEGRATION, utils.Network(2),
			utils.Network(20), utils.Network(100),
		}
		for _, n := range networks {
			t.Run(fmt.Sprintf("%d", n), func(t *testing.T) {
				cfg := &node.Config{Network: n}

				_, err := node.New(cfg)

				if utils.IsValidNetwork(cfg.Network) {
					assert.NoError(t, err)
				} else {
					assert.Error(t, err, utils.ErrUnknownNetwork)
				}
			})
		}
	})

	t.Run("default db-path", func(t *testing.T) {
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
				snNode, err := node.New(cfg)
				require.NoError(t, err)

				junoN, ok := snNode.(*node.Node)
				require.True(t, ok)
				assert.Equal(t, expectedCfg, junoN.Config())
			})
		}
	})
}
