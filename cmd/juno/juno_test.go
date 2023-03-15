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
	defaultDbPath := ""
	defaultNetwork := utils.MAINNET
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
				DatabasePath: defaultDbPath,
				Network:      defaultNetwork,
				Pprof:        defaultPprof,
			},
		},
		"config file path is empty string": {
			inputArgs: []string{"--config", ""},
			expectedConfig: &node.Config{
				LogLevel:     defaultLogLevel,
				RpcPort:      defaultRpcPort,
				DatabasePath: defaultDbPath,
				Network:      defaultNetwork,
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
				Network:  defaultNetwork,
			},
		},
		"config file with all settings but without any other flags": {
			cfgFile: true,
			cfgFileContents: `log-level: debug
rpc-port: 4576
db-path: /home/.juno
network: goerli2
pprof: true
`,
			expectedConfig: &node.Config{
				LogLevel:     utils.DEBUG,
				RpcPort:      4576,
				DatabasePath: "/home/.juno",
				Network:      utils.GOERLI2,
				Pprof:        true,
			},
		},
		"config file with some settings but without any other flags": {
			cfgFile: true,
			cfgFileContents: `log-level: debug
rpc-port: 4576
`,
			expectedConfig: &node.Config{
				LogLevel:     utils.DEBUG,
				RpcPort:      4576,
				DatabasePath: defaultDbPath,
				Network:      defaultNetwork,
				Pprof:        defaultPprof,
			},
		},
		"all flags without config file": {
			inputArgs: []string{
				"--log-level", "debug", "--rpc-port", "4576",
				"--db-path", "/home/.juno", "--network", "goerli", "--pprof",
			},
			expectedConfig: &node.Config{
				LogLevel:     utils.DEBUG,
				RpcPort:      4576,
				DatabasePath: "/home/.juno",
				Network:      utils.GOERLI,
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
				DatabasePath: "/home/.juno",
				Network:      utils.INTEGRATION,
			},
		},
		"all setting set in both config file and flags": {
			cfgFile: true,
			cfgFileContents: `log-level: debug
rpc-port: 4576
db-path: /home/config-file/.juno
network: goerli
pprof: true
`,
			inputArgs: []string{
				"--log-level", "error", "--rpc-port", "4577",
				"--db-path", "/home/flag/.juno", "--network", "integration", "--pprof",
			},
			expectedConfig: &node.Config{
				LogLevel:     utils.ERROR,
				RpcPort:      4577,
				DatabasePath: "/home/flag/.juno",
				Network:      utils.INTEGRATION,
				Pprof:        true,
			},
		},
		"some setting set in both config file and flags": {
			cfgFile: true,
			cfgFileContents: `log-level: warn
rpc-port: 4576
network: goerli
`,
			inputArgs: []string{"--db-path", "/home/flag/.juno"},
			expectedConfig: &node.Config{
				LogLevel:     utils.WARN,
				RpcPort:      4576,
				DatabasePath: "/home/flag/.juno",
				Network:      utils.GOERLI,
				Pprof:        defaultPprof,
			},
		},
		"some setting set in default, config file and flags": {
			cfgFile:         true,
			cfgFileContents: "network: goerli2",
			inputArgs:       []string{"--db-path", "/home/flag/.juno", "--pprof"},
			expectedConfig: &node.Config{
				LogLevel:     defaultLogLevel,
				RpcPort:      defaultRpcPort,
				DatabasePath: "/home/flag/.juno",
				Network:      utils.GOERLI2,
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
