package main_test

import (
	"context"
	"os"
	"testing"

	juno "github.com/NethermindEth/juno/cmd/juno"
	"github.com/NethermindEth/juno/node"
	"github.com/NethermindEth/juno/utils"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigPrecedence(t *testing.T) {
	// The purpose of these tests are to ensure the precedence of our config
	// values is respected. Since viper offers this feature, it would be
	// redundant to enumerate all combinations. Thus, only a select few are
	// tested for sanity. These tests are not intended to perform semantics
	// checks on the config, those will be checked by the StarknetNode
	// implementation.
	defaultLogLevel := utils.INFO
	defaultRpcPort := uint16(6060)
	defaultMetrics := false
	defaultDbPath := ""
	defaultNetwork := utils.MAINNET
	defaultEthNode := ""
	defaultPprof := false

	tests := map[string]struct {
		cfgFile         bool
		cfgFileContents string
		expectErr       bool
		inputArgs       []string
		expectedConfig  *node.Config
	}{
		"default config with no flags": {
			inputArgs: []string{""},
			expectedConfig: &node.Config{
				LogLevel:     defaultLogLevel,
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
				LogLevel:     defaultLogLevel,
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
			cfgFile:         true,
			cfgFileContents: "\n",
			expectedConfig: &node.Config{
				LogLevel: defaultLogLevel,
				RpcPort:  defaultRpcPort,
				Metrics:  defaultMetrics,
				Network:  defaultNetwork,
				EthNode:  defaultEthNode,
			},
		},
		"config file with all settings but without any other flags": {
			cfgFile: true,
			cfgFileContents: `log-level: debug
rpc-port: 4576
metrics: true
db-path: /home/.juno
network: goerli2
eth-node: "https://some-ethnode:5673"
pprof: true
`,
			expectedConfig: &node.Config{
				LogLevel:     utils.DEBUG,
				RpcPort:      4576,
				Metrics:      true,
				DatabasePath: "/home/.juno",
				Network:      utils.GOERLI2,
				EthNode:      "https://some-ethnode:5673",
				Pprof:        true,
			},
		},
		"config file with some settings but without any other flags": {
			cfgFile: true,
			cfgFileContents: `log-level: debug
rpc-port: 4576
metrics: true
`,
			expectedConfig: &node.Config{
				LogLevel:     utils.DEBUG,
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
				"--log-level", "debug", "--rpc-port", "4576",
				"--metrics", "--db-path", "/home/.juno", "--network", "goerli",
				"--eth-node", "https://some-ethnode:5673", "--pprof",
			},
			expectedConfig: &node.Config{
				LogLevel:     utils.DEBUG,
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
				"--log-level", "debug", "--rpc-port", "4576", "--db-path", "/home/.juno",
				"--network", "integration",
			},
			expectedConfig: &node.Config{
				LogLevel:     utils.DEBUG,
				RpcPort:      4576,
				Metrics:      defaultMetrics,
				DatabasePath: "/home/.juno",
				Network:      utils.INTEGRATION,
				EthNode:      defaultEthNode,
			},
		},
		"all setting set in both config file and flags": {
			cfgFile: true,
			cfgFileContents: `log-level: debug
rpc-port: 4576
metrics: true
db-path: /home/config-file/.juno
network: goerli
eth-node: "https://some-ethnode:5673"
pprof: true
`,
			inputArgs: []string{
				"--log-level", "error", "--rpc-port", "4577",
				"--metrics", "--db-path", "/home/flag/.juno", "--network", "integration",
				"--eth-node", "https://some-ethnode:5674", "--pprof",
			},
			expectedConfig: &node.Config{
				LogLevel:     utils.ERROR,
				RpcPort:      4577,
				Metrics:      true,
				DatabasePath: "/home/flag/.juno",
				Network:      utils.INTEGRATION,
				EthNode:      "https://some-ethnode:5674",
				Pprof:        true,
			},
		},
		"some setting set in both config file and flags": {
			cfgFile: true,
			cfgFileContents: `log-level: warn
rpc-port: 4576
network: goerli
`,
			inputArgs: []string{
				"--metrics", "--db-path", "/home/flag/.juno", "--eth-node",
				"https://some-ethnode:5674",
			},
			expectedConfig: &node.Config{
				LogLevel:     utils.WARN,
				RpcPort:      4576,
				Metrics:      true,
				DatabasePath: "/home/flag/.juno",
				Network:      utils.GOERLI,
				EthNode:      "https://some-ethnode:5674",
				Pprof:        defaultPprof,
			},
		},
		"some setting set in default, config file and flags": {
			cfgFile:         true,
			cfgFileContents: "network: goerli2",
			inputArgs: []string{
				"--metrics", "--db-path", "/home/flag/.juno", "--eth-node",
				"https://some-ethnode:5674", "--pprof",
			},
			expectedConfig: &node.Config{
				LogLevel:     defaultLogLevel,
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
			if tc.cfgFile {
				fileN, cleanup := tempCfgFile(t, tc.cfgFileContents)
				defer cleanup()
				tc.inputArgs = append(tc.inputArgs, "--config", fileN)
			}

			config := new(node.Config)
			cmd := juno.NewCmd(config, func(_ *cobra.Command, _ []string) error { return nil })
			cmd.SetArgs(tc.inputArgs)

			err := cmd.ExecuteContext(context.Background())
			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			assert.Equal(t, tc.expectedConfig, config)
		})
	}
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
