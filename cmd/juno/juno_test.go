package main_test

import (
	"bytes"
	"context"
	"os"
	"sync"
	"testing"

	juno "github.com/NethermindEth/juno/cmd/juno"
	"github.com/NethermindEth/juno/node"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type spyJuno struct {
	sync.RWMutex
	cfg   *node.Config
	calls []string
}

func newSpyJuno(junoCfg *node.Config) (node.StarknetNode, error) {
	return &spyJuno{cfg: junoCfg}, nil
}

func (s *spyJuno) Run(ctx context.Context) {
	s.Lock()
	s.calls = append(s.calls, "run")
	s.Unlock()
}

func (s *spyJuno) Config() node.Config {
	return *s.cfg
}

func TestNewCmd(t *testing.T) {
	t.Run("greeting", func(t *testing.T) {
		expected := `
       _                    
      | |                   
      | |_   _ _ __   ___   
  _   | | | | | '_ \ / _ \  
 | |__| | |_| | | | | (_) |  
  \____/ \__,_|_| |_|\___/  

Juno is a Go implementation of a Starknet full node client created by Nethermind.

`
		b := new(bytes.Buffer)

		cmd := juno.NewCmd(newSpyJuno)
		cmd.SetOut(b)
		err := cmd.ExecuteContext(context.Background())
		require.NoError(t, err)

		actual := b.String()
		assert.Equal(t, expected, actual)

		n, ok := juno.StarknetNode.(*spyJuno)
		require.Equal(t, true, ok)
		assert.Equal(t, []string{"run"}, n.calls)
	})

	t.Run("config precedence", func(t *testing.T) {
		// The purpose of these tests are to ensure the precedence of our config
		// values is respected. Since viper offers this feature, it would be
		// redundant to enumerate all combinations. Thus, only a select few are
		// tested for sanity. These tests are not intended to perform semantics
		// checks on the config, those will be checked by the StarknetNode
		// implementation.
		defaultVerbosity := utils.INFO
		defaultRpcPort := uint16(6060)
		defaultMetrics := false
		defaultDbPath := ""
		defaultNetwork := utils.MAINNET
		defaultEthNode := ""
		defaultPprof := false

		tests := map[string]struct {
			cfgFile         func(t *testing.T, cfg string) (string, func())
			cfgFileContents string
			expectErr       bool
			inputArgs       []string
			expectedConfig  *node.Config
		}{
			"default config with no flags": {
				inputArgs: []string{""},
				expectedConfig: &node.Config{
					Verbosity:    defaultVerbosity,
					RpcPort:      defaultRpcPort,
					Metrics:      defaultMetrics,
					DatabasePath: defaultDbPath,
					Network:      defaultNetwork,
					EthNode:      defaultEthNode,
					Pprof:        defaultPprof,
				},
			},
			"config file path is empty string": {
				inputArgs: []string{"--config", ""},
				expectedConfig: &node.Config{
					Verbosity:    defaultVerbosity,
					RpcPort:      defaultRpcPort,
					Metrics:      defaultMetrics,
					DatabasePath: defaultDbPath,
					Network:      defaultNetwork,
					EthNode:      defaultEthNode,
					Pprof:        defaultPprof,
				},
			},
			"config file doesn't exist": {
				inputArgs: []string{"--config", "config-file-test.yaml"},
				expectErr: true,
			},
			"config file contents are empty": {
				cfgFile:         tempCfgFile,
				cfgFileContents: "\n",
				expectedConfig: &node.Config{
					Verbosity: defaultVerbosity,
					RpcPort:   defaultRpcPort,
					Metrics:   defaultMetrics,
					Network:   defaultNetwork,
					EthNode:   defaultEthNode,
					Pprof:     defaultPprof,
				},
			},
			"config file with all settings but without any other flags": {
				cfgFile: tempCfgFile,
				cfgFileContents: `verbosity: 0
rpc-port: 4576
metrics: true
db-path: /home/.juno
network: 2
eth-node: "https://some-ethnode:5673"
pprof: true
`,
				expectedConfig: &node.Config{
					Verbosity:    utils.DEBUG,
					RpcPort:      4576,
					Metrics:      true,
					DatabasePath: "/home/.juno",
					Network:      utils.GOERLI2,
					EthNode:      "https://some-ethnode:5673",
					Pprof:        true,
				},
			},
			"config file with some settings but without any other flags": {
				cfgFile: tempCfgFile,
				cfgFileContents: `verbosity: 0
rpc-port: 4576
metrics: true
`,
				expectedConfig: &node.Config{
					Verbosity:    utils.DEBUG,
					RpcPort:      4576,
					Metrics:      true,
					DatabasePath: defaultDbPath,
					Network:      defaultNetwork,
					EthNode:      defaultEthNode,
					Pprof:        defaultPprof,
				},
			},
			"all flags without config file": {
				inputArgs: []string{
					"--verbosity", "0", "--rpc-port", "4576",
					"--metrics", "--db-path", "/home/.juno", "--network", "1",
					"--eth-node", "https://some-ethnode:5673", "--pprof", "true",
				},
				expectedConfig: &node.Config{
					Verbosity:    utils.DEBUG,
					RpcPort:      4576,
					Metrics:      true,
					DatabasePath: "/home/.juno",
					Network:      utils.GOERLI,
					EthNode:      "https://some-ethnode:5673",
					Pprof:        true,
				},
			},
			"some flags without config file": {
				inputArgs: []string{
					"--verbosity", "0", "--rpc-port", "4576", "--db-path", "/home/.juno",
					"--network", "3", "--pprof",
				},
				expectedConfig: &node.Config{
					Verbosity:    utils.DEBUG,
					RpcPort:      4576,
					Metrics:      defaultMetrics,
					DatabasePath: "/home/.juno",
					Network:      utils.INTEGRATION,
					EthNode:      defaultEthNode,
					Pprof:        true,
				},
			},
			"all setting set in both config file and flags": {
				cfgFile: tempCfgFile,
				cfgFileContents: `verbosity: 0
rpc-port: 4576
metrics: true
db-path: /home/config-file/.juno
network: 1
eth-node: "https://some-ethnode:5673"
pprof: true
`,
				inputArgs: []string{
					"--verbosity", "3", "--rpc-port", "4577",
					"--metrics", "--db-path", "/home/flag/.juno", "--network", "3",
					"--eth-node", "https://some-ethnode:5674", "--pprof",
				},
				expectedConfig: &node.Config{
					Verbosity:    utils.ERROR,
					RpcPort:      4577,
					Metrics:      true,
					DatabasePath: "/home/flag/.juno",
					Network:      utils.INTEGRATION,
					EthNode:      "https://some-ethnode:5674",
					Pprof:        true,
				},
			},
			"some setting set in both config file and flags": {
				cfgFile: tempCfgFile,
				cfgFileContents: `verbosity: 2
rpc-port: 4576
network: 1
`,
				inputArgs: []string{
					"--metrics", "--db-path", "/home/flag/.juno", "--eth-node",
					"https://some-ethnode:5674",
				},
				expectedConfig: &node.Config{
					Verbosity:    utils.WARN,
					RpcPort:      4576,
					Metrics:      true,
					DatabasePath: "/home/flag/.juno",
					Network:      utils.GOERLI,
					EthNode:      "https://some-ethnode:5674",
					Pprof:        false,
				},
			},
			"some setting set in default, config file and flags": {
				cfgFile:         tempCfgFile,
				cfgFileContents: `network: 2`,
				inputArgs: []string{
					"--metrics", "--db-path", "/home/flag/.juno", "--eth-node",
					"https://some-ethnode:5674", "--pprof", "true",
				},
				expectedConfig: &node.Config{
					Verbosity:    defaultVerbosity,
					RpcPort:      defaultRpcPort,
					Metrics:      true,
					DatabasePath: "/home/flag/.juno",
					Network:      utils.GOERLI2,
					EthNode:      "https://some-ethnode:5674",
					Pprof:        true,
				},
			},
		}

		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				if tc.cfgFile != nil {
					fileN, cleanup := tc.cfgFile(t, tc.cfgFileContents)
					defer cleanup()
					tc.inputArgs = append(tc.inputArgs, []string{"--config", fileN}...)
				}

				cmd := juno.NewCmd(newSpyJuno)
				cmd.SetArgs(tc.inputArgs)

				err := cmd.ExecuteContext(context.Background())
				if tc.expectErr {
					require.Error(t, err)
					return
				}
				require.NoError(t, err)

				n, ok := juno.StarknetNode.(*spyJuno)
				require.Equal(t, true, ok)
				assert.Equal(t, tc.expectedConfig, n.cfg)
				assert.Equal(t, []string{"run"}, n.calls)
			})
		}
	})
}

func tempCfgFile(t *testing.T, cfg string) (string, func()) {
	f, err := os.CreateTemp("", "junoCfg.*.yaml")
	require.NoError(t, err)

	defer func() {
		err = f.Close()
		require.NoError(t, err)
	}()

	_, err = f.WriteString(cfg)
	require.NoError(t, err)

	err = f.Sync()
	require.NoError(t, err)

	return f.Name(), func() {
		err = os.Remove(f.Name())
		require.NoError(t, err)
	}
}
