package juno

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/NethermindEth/juno/internal/utils"
	"gotest.tools/v3/assert"
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
					assert.NilError(t, err)
				} else {
					assert.Error(t, err, ErrUnknownNetwork.Error())
				}
			})
		}
	})

	t.Run("default db-path", func(t *testing.T) {
		defaultDataDir, err := utils.DefaultDataDir()
		assert.NilError(t, err)

		networks := []utils.Network{utils.GOERLI, utils.MAINNET}

		for _, n := range networks {
			t.Run(n.String(), func(t *testing.T) {
				cfg := &Config{Network: n, DatabasePath: ""}
				expectedCfg := &Config{
					Network:      n,
					DatabasePath: filepath.Join(defaultDataDir, n.String()),
				}
				node, err := New(cfg)
				assert.NilError(t, err)

				junoN, ok := node.(*Node)
				assert.Equal(t, true, ok)
				assert.DeepEqual(t, expectedCfg, junoN.cfg)
			})
		}
	})
}
