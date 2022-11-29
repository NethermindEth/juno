package node

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("network", func(t *testing.T) {
		networks := []utils.Network{
			utils.GOERLI, utils.MAINNET, utils.Network(2),
			utils.Network(20), utils.Network(100),
		}
		for _, n := range networks {
			t.Run(fmt.Sprintf("%d", n), func(t *testing.T) {
				cfg := &Config{Network: n}

				_, err := New(cfg)

				if cfg.Network == utils.GOERLI || cfg.Network == utils.MAINNET {
					assert.NoError(t, err)
				} else {
					assert.Error(t, err, ErrUnknownNetwork)
				}
			})
		}
	})

	t.Run("default db-path", func(t *testing.T) {
		defaultDataDir, err := utils.DefaultDataDir()
		require.NoError(t, err)

		networks := []utils.Network{utils.GOERLI, utils.MAINNET}

		for _, n := range networks {
			t.Run(n.String(), func(t *testing.T) {
				cfg := &Config{Network: n, DatabasePath: ""}
				expectedCfg := &Config{
					Network:      n,
					DatabasePath: filepath.Join(defaultDataDir, n.String()),
				}
				node, err := New(cfg)
				require.NoError(t, err)

				junoN, ok := node.(*Node)
				require.Equal(t, true, ok)
				assert.Equal(t, expectedCfg, junoN.cfg)
			})
		}
	})
}
