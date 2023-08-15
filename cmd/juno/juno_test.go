package main_test

import (
	"context"
	"os"
	"testing"
	"time"

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
	defaultHTTP := false
	defaultHTTPPort := uint16(6060)
	defaultWS := false
	defaultWSPort := uint16(6061)
	defaultDBPath := ""
	defaultNetwork := utils.MAINNET
	defaultPprof := false
	defaultPprofPort := uint16(6062)
	defaultMetrics := false
	defaultMetricsPort := uint16(6063)
	defaultGRPC := false
	defaultGRPCPort := uint16(6064)
	defaultColour := true
	defaultPendingPollInterval := time.Duration(0)

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
				LogLevel:            defaultLogLevel,
				HTTP:                defaultHTTP,
				HTTPPort:            defaultHTTPPort,
				Websocket:           defaultWS,
				WebsocketPort:       defaultWSPort,
				DatabasePath:        defaultDBPath,
				Network:             defaultNetwork,
				Pprof:               defaultPprof,
				PprofPort:           defaultPprofPort,
				GRPC:                defaultGRPC,
				GRPCPort:            defaultGRPCPort,
				Metrics:             defaultMetrics,
				MetricsPort:         defaultMetricsPort,
				Colour:              defaultColour,
				PendingPollInterval: defaultPendingPollInterval,
			},
		},
		"config file path is empty string": {
			inputArgs: []string{"--config", ""},
			expectedConfig: &node.Config{
				LogLevel:            defaultLogLevel,
				HTTP:                defaultHTTP,
				HTTPPort:            defaultHTTPPort,
				Websocket:           defaultWS,
				WebsocketPort:       defaultWSPort,
				GRPC:                defaultGRPC,
				GRPCPort:            defaultGRPCPort,
				Metrics:             defaultMetrics,
				MetricsPort:         defaultMetricsPort,
				DatabasePath:        defaultDBPath,
				Network:             defaultNetwork,
				Pprof:               defaultPprof,
				PprofPort:           defaultPprofPort,
				Colour:              defaultColour,
				PendingPollInterval: defaultPendingPollInterval,
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
				LogLevel:            defaultLogLevel,
				HTTP:                defaultHTTP,
				HTTPPort:            defaultHTTPPort,
				Websocket:           defaultWS,
				WebsocketPort:       defaultWSPort,
				GRPC:                defaultGRPC,
				GRPCPort:            defaultGRPCPort,
				Metrics:             defaultMetrics,
				MetricsPort:         defaultMetricsPort,
				Network:             defaultNetwork,
				Colour:              defaultColour,
				PendingPollInterval: defaultPendingPollInterval,
				Pprof:               defaultPprof,
				PprofPort:           defaultPprofPort,
			},
		},
		"config file with all settings but without any other flags": {
			cfgFile: true,
			cfgFileContents: `log-level: debug
http-port: 4576
db-path: /home/.juno
network: goerli2
pprof: true
`,
			expectedConfig: &node.Config{
				LogLevel:            utils.DEBUG,
				HTTP:                defaultHTTP,
				HTTPPort:            4576,
				Websocket:           defaultWS,
				WebsocketPort:       defaultWSPort,
				GRPC:                defaultGRPC,
				GRPCPort:            defaultGRPCPort,
				Metrics:             defaultMetrics,
				MetricsPort:         defaultMetricsPort,
				DatabasePath:        "/home/.juno",
				Network:             utils.GOERLI2,
				Pprof:               true,
				PprofPort:           defaultPprofPort,
				Colour:              defaultColour,
				PendingPollInterval: defaultPendingPollInterval,
			},
		},
		"config file with some settings but without any other flags": {
			cfgFile: true,
			cfgFileContents: `log-level: debug
http-port: 4576
`,
			expectedConfig: &node.Config{
				LogLevel:            utils.DEBUG,
				HTTP:                defaultHTTP,
				HTTPPort:            4576,
				Websocket:           defaultWS,
				WebsocketPort:       defaultWSPort,
				GRPC:                defaultGRPC,
				GRPCPort:            defaultGRPCPort,
				Metrics:             defaultMetrics,
				MetricsPort:         defaultMetricsPort,
				DatabasePath:        defaultDBPath,
				Network:             defaultNetwork,
				Pprof:               defaultPprof,
				PprofPort:           defaultPprofPort,
				Colour:              defaultColour,
				PendingPollInterval: defaultPendingPollInterval,
			},
		},
		"all flags without config file": {
			inputArgs: []string{
				"--log-level", "debug", "--http-port", "4576",
				"--db-path", "/home/.juno", "--network", "goerli", "--pprof",
			},
			expectedConfig: &node.Config{
				LogLevel:      utils.DEBUG,
				HTTP:          defaultHTTP,
				HTTPPort:      4576,
				Websocket:     defaultWS,
				WebsocketPort: defaultWSPort,
				GRPC:          defaultGRPC,
				GRPCPort:      defaultGRPCPort,
				Metrics:       defaultMetrics,
				MetricsPort:   defaultMetricsPort,
				DatabasePath:  "/home/.juno",
				Network:       utils.GOERLI,
				Pprof:         true,
				PprofPort:     defaultPprofPort,
				Colour:        defaultColour,
			},
		},
		"some flags without config file": {
			inputArgs: []string{
				"--log-level", "debug", "--http-port", "4576", "--db-path", "/home/.juno",
				"--network", "integration",
			},
			expectedConfig: &node.Config{
				LogLevel:            utils.DEBUG,
				HTTP:                defaultHTTP,
				HTTPPort:            4576,
				Websocket:           defaultWS,
				WebsocketPort:       defaultWSPort,
				GRPC:                defaultGRPC,
				GRPCPort:            defaultGRPCPort,
				Metrics:             defaultMetrics,
				MetricsPort:         defaultMetricsPort,
				DatabasePath:        "/home/.juno",
				Network:             utils.INTEGRATION,
				Pprof:               defaultPprof,
				PprofPort:           defaultPprofPort,
				Colour:              defaultColour,
				PendingPollInterval: defaultPendingPollInterval,
			},
		},
		"all setting set in both config file and flags": {
			cfgFile: true,
			cfgFileContents: `log-level: debug
http: true
http-port: 4576
ws: true
ws-port: 4576
metrics: true
metrics-port: 4576
grpc: true
grpc-port: 4576
db-path: /home/config-file/.juno
network: goerli
pprof: true
pprof-port: 6064
pending-poll-interval: 5s
`,
			inputArgs: []string{
				"--log-level", "error", "--http", "--http-port", "4577", "--ws", "--ws-port", "4577",
				"--grpc", "--grpc-port", "4577", "--metrics", "--metrics-port", "4577",
				"--db-path", "/home/flag/.juno", "--network", "integration", "--pprof", "--pending-poll-interval", time.Millisecond.String(),
			},
			expectedConfig: &node.Config{
				LogLevel:            utils.ERROR,
				HTTP:                true,
				HTTPPort:            4577,
				Websocket:           true,
				WebsocketPort:       4577,
				Metrics:             true,
				MetricsPort:         4577,
				GRPC:                true,
				GRPCPort:            4577,
				DatabasePath:        "/home/flag/.juno",
				Network:             utils.INTEGRATION,
				Pprof:               true,
				PprofPort:           6064,
				Colour:              defaultColour,
				PendingPollInterval: time.Millisecond,
			},
		},
		"some setting set in both config file and flags": {
			cfgFile: true,
			cfgFileContents: `log-level: warn
http-port: 4576
network: goerli
`,
			inputArgs: []string{"--db-path", "/home/flag/.juno"},
			expectedConfig: &node.Config{
				LogLevel:            utils.WARN,
				HTTP:                defaultHTTP,
				HTTPPort:            4576,
				Websocket:           defaultWS,
				WebsocketPort:       defaultWSPort,
				GRPC:                defaultGRPC,
				GRPCPort:            defaultGRPCPort,
				Metrics:             defaultMetrics,
				MetricsPort:         defaultMetricsPort,
				DatabasePath:        "/home/flag/.juno",
				Network:             utils.GOERLI,
				Pprof:               defaultPprof,
				PprofPort:           defaultPprofPort,
				Colour:              defaultColour,
				PendingPollInterval: defaultPendingPollInterval,
			},
		},
		"some setting set in default, config file and flags": {
			cfgFile:         true,
			cfgFileContents: `network: goerli2`,
			inputArgs:       []string{"--db-path", "/home/flag/.juno", "--pprof"},
			expectedConfig: &node.Config{
				LogLevel:            defaultLogLevel,
				HTTP:                defaultHTTP,
				HTTPPort:            defaultHTTPPort,
				Websocket:           defaultWS,
				WebsocketPort:       defaultWSPort,
				GRPC:                defaultGRPC,
				GRPCPort:            defaultGRPCPort,
				Metrics:             defaultMetrics,
				MetricsPort:         defaultMetricsPort,
				DatabasePath:        "/home/flag/.juno",
				Network:             utils.GOERLI2,
				Pprof:               true,
				PprofPort:           defaultPprofPort,
				Colour:              defaultColour,
				PendingPollInterval: defaultPendingPollInterval,
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.cfgFile {
				fileN := tempCfgFile(t, tc.cfgFileContents)
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

func tempCfgFile(t *testing.T, cfg string) string {
	t.Helper()

	f, err := os.CreateTemp(t.TempDir(), "junoCfg.*.yaml")
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, f.Close())
	})

	_, err = f.WriteString(cfg)
	require.NoError(t, err)

	require.NoError(t, f.Sync())

	return f.Name()
}
