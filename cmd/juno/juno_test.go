package juno_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	junoCmd "github.com/NethermindEth/juno/cmd/juno"
	"github.com/NethermindEth/juno/internal/config"
	"github.com/NethermindEth/juno/internal/juno"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type spyJuno struct {
	cfg   *config.Juno
	calls []string
}

func newSpyJuno(junoCfg *config.Juno) (juno.StarkNetNode, error) {
	return &spyJuno{cfg: junoCfg}, nil
}

func (s *spyJuno) Run() error {
	s.calls = append(s.calls, "run")
	return nil
}

func (s *spyJuno) Shutdown() error {
	s.calls = append(s.calls, "shutdown")
	return nil
}

func TestNewJunoCmd(t *testing.T) {
	t.Run("greeting", func(t *testing.T) {
		expected := `
       _                    
      | |                   
      | |_   _ _ __   ___   
  _   | | | | | '_ \ / _ \  
 | |__| | |_| | | | | (_) |  
  \____/ \__,_|_| |_|\___/  

Juno is a Go implementation of a StarkNet full node client made with ❤️ by Nethermind.

`
		b := new(bytes.Buffer)

		cmd := junoCmd.NewCmd(newSpyJuno)
		cmd.SetOut(b)
		err := cmd.Execute()
		require.NoError(t, err)

		actual := b.String()
		assert.Equal(t, expected, actual)
	})

	t.Run("config precedence", func(t *testing.T) {
		// The purpose of these tests are to ensure the precedence of our config
		// values is respected. Since viper offers this feature, it would be
		// redundant to enumerate all combinations. Thus, only a select few are
		// tested for sanity.

		dbPathFn := func(suffix string) string {
			homeDir, err := os.UserHomeDir()
			require.NoError(t, err)
			return filepath.Join(homeDir, ".juno", suffix)
		}
		defaultVerbosity := "info"
		defaultRpcPort := uint16(6060)
		defaultMetricsPort := uint16(0)
		defaultNetwork := config.GOERLI
		defaultEthNode := ""

		tests := map[string]struct {
			inputArgs      []string
			expectedConfig *config.Juno
		}{
			"default config no flags": {
				inputArgs: []string{""},
				expectedConfig: &config.Juno{
					Verbosity:    defaultVerbosity,
					RpcPort:      defaultRpcPort,
					MetricsPort:  defaultMetricsPort,
					DatabasePath: dbPathFn(config.GOERLI.String()),
					Network:      defaultNetwork, EthNode: defaultEthNode,
				},
			},
		}

		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				cmd := junoCmd.NewCmd(newSpyJuno)
				cmd.SetArgs(tc.inputArgs)

				err := cmd.Execute()
				require.NoError(t, err)

				n, ok := junoCmd.JunoNode.(*spyJuno)
				require.Equal(t, true, ok)
				assert.Equal(t, tc.expectedConfig, n.cfg)
			})
		}
	})
}
