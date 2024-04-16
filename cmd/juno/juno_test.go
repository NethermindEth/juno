package main_test

import (
	"context"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	juno "github.com/NethermindEth/juno/cmd/juno"
	"github.com/NethermindEth/juno/node"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigPrecedence(t *testing.T) {
	pwd, err := os.Getwd()
	require.NoError(t, err)

	// The purpose of these tests is to ensure the precedence of our config
	// values is respected. Since viper offers this feature, it would be
	// redundant to enumerate all combinations. Thus, only a select few are
	// tested for sanity. These tests are not intended to perform semantics
	// checks on the config, those will be checked by the node implementation.
	defaultHost := "localhost"
	defaultLogLevel := utils.INFO
	defaultHTTP := false
	defaultHTTPPort := uint16(6060)
	defaultWS := false
	defaultWSPort := uint16(6061)
	defaultDBPath := filepath.Join(pwd, "juno")
	defaultCoreContractAddress := common.HexToAddress("0xc662c410C0ECf747543f5bA90660f6ABeBD9C8c4")
	defaultNetwork := utils.Mainnet
	defaultCustomNetwork := utils.Network{
		Name:                "custom",
		FeederURL:           "awesome_feeder_url",
		GatewayURL:          "awesome_gateway_url",
		L2ChainID:           "SN_AWESOME",
		L1ChainID:           new(big.Int).SetUint64(1),
		CoreContractAddress: defaultCoreContractAddress,
		BlockHashMetaInfo: &utils.BlockHashMetaInfo{
			First07Block:      0,
			UnverifiableRange: []uint64{0, 10},
		},
	}
	defaultPprof := false
	defaultPprofPort := uint16(6062)
	defaultMetrics := false
	defaultMetricsPort := uint16(9090)
	defaultGRPC := false
	defaultGRPCPort := uint16(6064)
	defaultColour := true
	defaultPendingPollInterval := time.Duration(0)
	defaultMaxVMs := uint(3 * runtime.GOMAXPROCS(0))
	defaultRPCMaxBlockScan := uint(math.MaxUint)
	defaultMaxCacheSize := uint(8)
	defaultMaxHandles := 1024
	defaultCallMaxSteps := uint(4_000_000)
	defaultGwTimeout := 5 * time.Second

	tests := map[string]struct {
		cfgFile         bool
		cfgFileContents string
		expectErr       bool
		inputArgs       []string
		env             []string
		expectedConfig  *node.Config
	}{
		"custom network all flags": {
			inputArgs: []string{
				"--log-level", "debug", "--http-port", "4576", "--http-host", "0.0.0.0",
				"--db-path", "/home/.juno", "--pprof", "--db-cache-size", "8",
				"--cn-name", "custom", "--cn-feeder-url", "awesome_feeder_url", "--cn-gateway-url", "awesome_gateway_url",
				"--cn-l1-chain-id", "0x1", "--cn-l2-chain-id", "SN_AWESOME",
				"--cn-unverifiable-range", "0,10",
				"--cn-core-contract-address", "0xc662c410C0ECf747543f5bA90660f6ABeBD9C8c4",
			},
			expectedConfig: &node.Config{
				LogLevel:            utils.DEBUG,
				HTTP:                defaultHTTP,
				HTTPHost:            "0.0.0.0",
				HTTPPort:            4576,
				Websocket:           defaultWS,
				WebsocketHost:       defaultHost,
				WebsocketPort:       defaultWSPort,
				GRPC:                defaultGRPC,
				GRPCHost:            defaultHost,
				GRPCPort:            defaultGRPCPort,
				Metrics:             defaultMetrics,
				MetricsHost:         defaultHost,
				MetricsPort:         defaultMetricsPort,
				DatabasePath:        "/home/.juno",
				Network:             defaultCustomNetwork,
				Pprof:               true,
				PprofHost:           defaultHost,
				PprofPort:           defaultPprofPort,
				Colour:              defaultColour,
				PendingPollInterval: defaultPendingPollInterval,
				MaxVMs:              defaultMaxVMs,
				MaxVMQueue:          2 * defaultMaxVMs,
				RPCMaxBlockScan:     defaultRPCMaxBlockScan,
				DBCacheSize:         defaultMaxCacheSize,
				DBMaxHandles:        defaultMaxHandles,
				RPCCallMaxSteps:     defaultCallMaxSteps,
				GatewayTimeout:      defaultGwTimeout,
			},
		},
		"custom network config file": {
			cfgFile: true,
			cfgFileContents: `log-level: debug
http-host: 0.0.0.0
http-port: 4576
db-path: /home/.juno
pprof: true
cn-name: custom
cn-feeder-url: awesome_feeder_url
cn-gateway-url: awesome_gateway_url
cn-l2-chain-id: SN_AWESOME
cn-l1-chain-id: 0x1
cn-core-contract-address: 0xc662c410C0ECf747543f5bA90660f6ABeBD9C8c4
cn-unverifiable-range: [0,10]
`,
			expectedConfig: &node.Config{
				LogLevel:            utils.DEBUG,
				HTTP:                defaultHTTP,
				HTTPHost:            "0.0.0.0",
				HTTPPort:            4576,
				Websocket:           defaultWS,
				WebsocketHost:       defaultHost,
				WebsocketPort:       defaultWSPort,
				GRPC:                defaultGRPC,
				GRPCHost:            defaultHost,
				GRPCPort:            defaultGRPCPort,
				Metrics:             defaultMetrics,
				MetricsHost:         defaultHost,
				MetricsPort:         defaultMetricsPort,
				DatabasePath:        "/home/.juno",
				Network:             defaultCustomNetwork,
				Pprof:               true,
				PprofHost:           defaultHost,
				PprofPort:           defaultPprofPort,
				Colour:              defaultColour,
				PendingPollInterval: defaultPendingPollInterval,
				MaxVMs:              defaultMaxVMs,
				MaxVMQueue:          2 * defaultMaxVMs,
				RPCMaxBlockScan:     defaultRPCMaxBlockScan,
				DBCacheSize:         defaultMaxCacheSize,
				DBMaxHandles:        defaultMaxHandles,
				RPCCallMaxSteps:     defaultCallMaxSteps,
				GatewayTimeout:      defaultGwTimeout,
			},
		},
		"default config with no flags": {
			inputArgs: []string{""},
			expectedConfig: &node.Config{
				LogLevel:            defaultLogLevel,
				HTTP:                defaultHTTP,
				HTTPHost:            defaultHost,
				HTTPPort:            defaultHTTPPort,
				Websocket:           defaultWS,
				WebsocketHost:       defaultHost,
				WebsocketPort:       defaultWSPort,
				DatabasePath:        defaultDBPath,
				Network:             defaultNetwork,
				Pprof:               defaultPprof,
				PprofHost:           defaultHost,
				PprofPort:           defaultPprofPort,
				GRPC:                defaultGRPC,
				GRPCHost:            defaultHost,
				GRPCPort:            defaultGRPCPort,
				Metrics:             defaultMetrics,
				MetricsHost:         defaultHost,
				MetricsPort:         defaultMetricsPort,
				Colour:              defaultColour,
				PendingPollInterval: defaultPendingPollInterval,
				MaxVMs:              defaultMaxVMs,
				MaxVMQueue:          2 * defaultMaxVMs,
				RPCMaxBlockScan:     defaultRPCMaxBlockScan,
				DBCacheSize:         defaultMaxCacheSize,
				DBMaxHandles:        defaultMaxHandles,
				RPCCallMaxSteps:     defaultCallMaxSteps,
				GatewayTimeout:      defaultGwTimeout,
			},
		},
		"config file path is empty string": {
			inputArgs: []string{"--config", ""},
			expectedConfig: &node.Config{
				LogLevel:            defaultLogLevel,
				HTTP:                defaultHTTP,
				HTTPHost:            defaultHost,
				HTTPPort:            defaultHTTPPort,
				Websocket:           defaultWS,
				WebsocketHost:       defaultHost,
				WebsocketPort:       defaultWSPort,
				GRPC:                defaultGRPC,
				GRPCHost:            defaultHost,
				GRPCPort:            defaultGRPCPort,
				Metrics:             defaultMetrics,
				MetricsHost:         defaultHost,
				MetricsPort:         defaultMetricsPort,
				DatabasePath:        defaultDBPath,
				Network:             defaultNetwork,
				Pprof:               defaultPprof,
				PprofHost:           defaultHost,
				PprofPort:           defaultPprofPort,
				Colour:              defaultColour,
				PendingPollInterval: defaultPendingPollInterval,
				MaxVMs:              defaultMaxVMs,
				MaxVMQueue:          2 * defaultMaxVMs,
				RPCMaxBlockScan:     defaultRPCMaxBlockScan,
				DBCacheSize:         defaultMaxCacheSize,
				DBMaxHandles:        defaultMaxHandles,
				RPCCallMaxSteps:     defaultCallMaxSteps,
				GatewayTimeout:      defaultGwTimeout,
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
				HTTPHost:            defaultHost,
				HTTPPort:            defaultHTTPPort,
				Websocket:           defaultWS,
				WebsocketHost:       defaultHost,
				WebsocketPort:       defaultWSPort,
				GRPC:                defaultGRPC,
				GRPCHost:            defaultHost,
				GRPCPort:            defaultGRPCPort,
				Metrics:             defaultMetrics,
				MetricsHost:         defaultHost,
				MetricsPort:         defaultMetricsPort,
				Network:             defaultNetwork,
				Colour:              defaultColour,
				PendingPollInterval: defaultPendingPollInterval,
				Pprof:               defaultPprof,
				PprofHost:           defaultHost,
				PprofPort:           defaultPprofPort,
				DatabasePath:        defaultDBPath,
				MaxVMs:              defaultMaxVMs,
				MaxVMQueue:          2 * defaultMaxVMs,
				RPCMaxBlockScan:     defaultRPCMaxBlockScan,
				DBCacheSize:         defaultMaxCacheSize,
				DBMaxHandles:        defaultMaxHandles,
				RPCCallMaxSteps:     defaultCallMaxSteps,
				GatewayTimeout:      defaultGwTimeout,
			},
		},
		"config file with all settings but without any other flags": {
			cfgFile: true,
			cfgFileContents: `log-level: debug
http-host: 0.0.0.0
http-port: 4576
db-path: /home/.juno
network: goerli2
pprof: true
`,
			expectedConfig: &node.Config{
				LogLevel:            utils.DEBUG,
				HTTP:                defaultHTTP,
				HTTPHost:            "0.0.0.0",
				HTTPPort:            4576,
				Websocket:           defaultWS,
				WebsocketHost:       defaultHost,
				WebsocketPort:       defaultWSPort,
				GRPC:                defaultGRPC,
				GRPCHost:            defaultHost,
				GRPCPort:            defaultGRPCPort,
				Metrics:             defaultMetrics,
				MetricsHost:         defaultHost,
				MetricsPort:         defaultMetricsPort,
				DatabasePath:        "/home/.juno",
				Network:             utils.Goerli2,
				Pprof:               true,
				PprofHost:           defaultHost,
				PprofPort:           defaultPprofPort,
				Colour:              defaultColour,
				PendingPollInterval: defaultPendingPollInterval,
				MaxVMs:              defaultMaxVMs,
				MaxVMQueue:          2 * defaultMaxVMs,
				RPCMaxBlockScan:     defaultRPCMaxBlockScan,
				DBCacheSize:         defaultMaxCacheSize,
				DBMaxHandles:        defaultMaxHandles,
				RPCCallMaxSteps:     defaultCallMaxSteps,
				GatewayTimeout:      defaultGwTimeout,
			},
		},
		"config file with some settings but without any other flags": {
			cfgFile: true,
			cfgFileContents: `log-level: debug
http-host: 0.0.0.0
http-port: 4576
`,
			expectedConfig: &node.Config{
				LogLevel:            utils.DEBUG,
				HTTP:                defaultHTTP,
				HTTPHost:            "0.0.0.0",
				HTTPPort:            4576,
				Websocket:           defaultWS,
				WebsocketHost:       defaultHost,
				WebsocketPort:       defaultWSPort,
				GRPC:                defaultGRPC,
				GRPCHost:            defaultHost,
				GRPCPort:            defaultGRPCPort,
				Metrics:             defaultMetrics,
				MetricsHost:         defaultHost,
				MetricsPort:         defaultMetricsPort,
				DatabasePath:        defaultDBPath,
				Network:             defaultNetwork,
				Pprof:               defaultPprof,
				PprofHost:           defaultHost,
				PprofPort:           defaultPprofPort,
				Colour:              defaultColour,
				PendingPollInterval: defaultPendingPollInterval,
				MaxVMs:              defaultMaxVMs,
				MaxVMQueue:          2 * defaultMaxVMs,
				RPCMaxBlockScan:     defaultRPCMaxBlockScan,
				DBCacheSize:         defaultMaxCacheSize,
				DBMaxHandles:        defaultMaxHandles,
				RPCCallMaxSteps:     defaultCallMaxSteps,
				GatewayTimeout:      defaultGwTimeout,
			},
		},
		"all flags without config file": {
			inputArgs: []string{
				"--log-level", "debug", "--http-port", "4576", "--http-host", "0.0.0.0",
				"--db-path", "/home/.juno", "--network", "goerli", "--pprof", "--db-cache-size", "8",
			},
			expectedConfig: &node.Config{
				LogLevel:        utils.DEBUG,
				HTTP:            defaultHTTP,
				HTTPHost:        "0.0.0.0",
				HTTPPort:        4576,
				Websocket:       defaultWS,
				WebsocketHost:   defaultHost,
				WebsocketPort:   defaultWSPort,
				GRPC:            defaultGRPC,
				GRPCHost:        defaultHost,
				GRPCPort:        defaultGRPCPort,
				Metrics:         defaultMetrics,
				MetricsHost:     defaultHost,
				MetricsPort:     defaultMetricsPort,
				DatabasePath:    "/home/.juno",
				Network:         utils.Goerli,
				Pprof:           true,
				PprofHost:       defaultHost,
				PprofPort:       defaultPprofPort,
				Colour:          defaultColour,
				MaxVMs:          defaultMaxVMs,
				MaxVMQueue:      2 * defaultMaxVMs,
				RPCMaxBlockScan: defaultRPCMaxBlockScan,
				DBCacheSize:     defaultMaxCacheSize,
				DBMaxHandles:    defaultMaxHandles,
				RPCCallMaxSteps: defaultCallMaxSteps,
				GatewayTimeout:  defaultGwTimeout,
			},
		},
		"some flags without config file": {
			inputArgs: []string{
				"--log-level", "debug", "--http-port", "4576", "--http-host", "0.0.0.0", "--db-path", "/home/.juno",
				"--network", "integration",
			},
			expectedConfig: &node.Config{
				LogLevel:            utils.DEBUG,
				HTTP:                defaultHTTP,
				HTTPHost:            "0.0.0.0",
				HTTPPort:            4576,
				Websocket:           defaultWS,
				WebsocketHost:       defaultHost,
				WebsocketPort:       defaultWSPort,
				GRPC:                defaultGRPC,
				GRPCHost:            defaultHost,
				GRPCPort:            defaultGRPCPort,
				Metrics:             defaultMetrics,
				MetricsHost:         defaultHost,
				MetricsPort:         defaultMetricsPort,
				DatabasePath:        "/home/.juno",
				Network:             utils.Integration,
				Pprof:               defaultPprof,
				PprofHost:           defaultHost,
				PprofPort:           defaultPprofPort,
				Colour:              defaultColour,
				PendingPollInterval: defaultPendingPollInterval,
				MaxVMs:              defaultMaxVMs,
				MaxVMQueue:          2 * defaultMaxVMs,
				RPCMaxBlockScan:     defaultRPCMaxBlockScan,
				DBCacheSize:         defaultMaxCacheSize,
				DBMaxHandles:        defaultMaxHandles,
				RPCCallMaxSteps:     defaultCallMaxSteps,
				GatewayTimeout:      defaultGwTimeout,
			},
		},
		"all setting set in both config file and flags": {
			cfgFile: true,
			cfgFileContents: `log-level: debug
http: true
http-host: 0.0.0.0
http-port: 4576
ws: true
ws-host: 0.0.0.0
ws-port: 4576
metrics: true
metrics-host: 0.0.0.0
metrics-port: 4576
grpc: true
grpc-host: 0.0.0.0
grpc-port: 4576
db-path: /home/config-file/.juno
network: goerli
pprof: true
pprof-host: 0.0.0.0
pprof-port: 6064
pending-poll-interval: 5s
db-cache-size: 8
`,
			inputArgs: []string{
				"--log-level", "error", "--http", "--http-port", "4577", "--http-host", "127.0.0.1", "--ws", "--ws-port", "4577", "--ws-host", "127.0.0.1",
				"--grpc", "--grpc-port", "4577", "--grpc-host", "127.0.0.1", "--metrics", "--metrics-port", "4577", "--metrics-host", "127.0.0.1",
				"--db-path", "/home/flag/.juno", "--network", "integration", "--pprof", "--pending-poll-interval", time.Millisecond.String(),
				"--db-cache-size", "9",
			},
			expectedConfig: &node.Config{
				LogLevel:            utils.ERROR,
				HTTP:                true,
				HTTPHost:            "127.0.0.1",
				HTTPPort:            4577,
				Websocket:           true,
				WebsocketHost:       "127.0.0.1",
				WebsocketPort:       4577,
				Metrics:             true,
				MetricsHost:         "127.0.0.1",
				MetricsPort:         4577,
				GRPC:                true,
				GRPCHost:            "127.0.0.1",
				GRPCPort:            4577,
				DatabasePath:        "/home/flag/.juno",
				Network:             utils.Integration,
				Pprof:               true,
				PprofHost:           "0.0.0.0",
				PprofPort:           6064,
				Colour:              defaultColour,
				PendingPollInterval: time.Millisecond,
				MaxVMs:              defaultMaxVMs,
				MaxVMQueue:          2 * defaultMaxVMs,
				RPCMaxBlockScan:     defaultRPCMaxBlockScan,
				DBCacheSize:         9,
				DBMaxHandles:        defaultMaxHandles,
				RPCCallMaxSteps:     defaultCallMaxSteps,
				GatewayTimeout:      defaultGwTimeout,
			},
		},
		"some setting set in both config file and flags": {
			cfgFile: true,
			cfgFileContents: `log-level: warn
http-host: 0.0.0.0
http-port: 4576
network: goerli
`,
			inputArgs: []string{"--db-path", "/home/flag/.juno"},
			expectedConfig: &node.Config{
				LogLevel:            utils.WARN,
				HTTP:                defaultHTTP,
				HTTPHost:            "0.0.0.0",
				HTTPPort:            4576,
				Websocket:           defaultWS,
				WebsocketHost:       defaultHost,
				WebsocketPort:       defaultWSPort,
				GRPC:                defaultGRPC,
				GRPCHost:            defaultHost,
				GRPCPort:            defaultGRPCPort,
				Metrics:             defaultMetrics,
				MetricsHost:         defaultHost,
				MetricsPort:         defaultMetricsPort,
				DatabasePath:        "/home/flag/.juno",
				Network:             utils.Goerli,
				Pprof:               defaultPprof,
				PprofHost:           defaultHost,
				PprofPort:           defaultPprofPort,
				Colour:              defaultColour,
				PendingPollInterval: defaultPendingPollInterval,
				MaxVMs:              defaultMaxVMs,
				MaxVMQueue:          2 * defaultMaxVMs,
				RPCMaxBlockScan:     defaultRPCMaxBlockScan,
				DBCacheSize:         defaultMaxCacheSize,
				DBMaxHandles:        defaultMaxHandles,
				RPCCallMaxSteps:     defaultCallMaxSteps,
				GatewayTimeout:      defaultGwTimeout,
			},
		},
		"some setting set in default, config file and flags": {
			cfgFile:         true,
			cfgFileContents: `network: goerli2`,
			inputArgs:       []string{"--db-path", "/home/flag/.juno", "--pprof"},
			expectedConfig: &node.Config{
				LogLevel:            defaultLogLevel,
				HTTP:                defaultHTTP,
				HTTPHost:            defaultHost,
				HTTPPort:            defaultHTTPPort,
				Websocket:           defaultWS,
				WebsocketHost:       defaultHost,
				WebsocketPort:       defaultWSPort,
				GRPC:                defaultGRPC,
				GRPCHost:            defaultHost,
				GRPCPort:            defaultGRPCPort,
				Metrics:             defaultMetrics,
				MetricsHost:         defaultHost,
				MetricsPort:         defaultMetricsPort,
				DatabasePath:        "/home/flag/.juno",
				Network:             utils.Goerli2,
				Pprof:               true,
				PprofHost:           defaultHost,
				PprofPort:           defaultPprofPort,
				Colour:              defaultColour,
				PendingPollInterval: defaultPendingPollInterval,
				MaxVMs:              defaultMaxVMs,
				MaxVMQueue:          2 * defaultMaxVMs,
				RPCMaxBlockScan:     defaultRPCMaxBlockScan,
				DBCacheSize:         defaultMaxCacheSize,
				DBMaxHandles:        defaultMaxHandles,
				RPCCallMaxSteps:     defaultCallMaxSteps,
				GatewayTimeout:      defaultGwTimeout,
			},
		},
		"only set env variables": {
			env: []string{"JUNO_HTTP_PORT", "8080", "JUNO_WS", "true", "JUNO_HTTP_HOST", "0.0.0.0"},
			expectedConfig: &node.Config{
				LogLevel:            defaultLogLevel,
				HTTP:                defaultHTTP,
				HTTPHost:            "0.0.0.0",
				HTTPPort:            8080,
				Websocket:           true,
				WebsocketHost:       defaultHost,
				WebsocketPort:       defaultWSPort,
				GRPC:                defaultGRPC,
				GRPCHost:            defaultHost,
				GRPCPort:            defaultGRPCPort,
				Metrics:             defaultMetrics,
				MetricsHost:         defaultHost,
				MetricsPort:         defaultMetricsPort,
				DatabasePath:        defaultDBPath,
				Network:             defaultNetwork,
				Pprof:               defaultPprof,
				PprofHost:           defaultHost,
				PprofPort:           defaultPprofPort,
				Colour:              defaultColour,
				PendingPollInterval: defaultPendingPollInterval,
				MaxVMs:              defaultMaxVMs,
				MaxVMQueue:          2 * defaultMaxVMs,
				RPCMaxBlockScan:     defaultRPCMaxBlockScan,
				DBCacheSize:         defaultMaxCacheSize,
				DBMaxHandles:        defaultMaxHandles,
				RPCCallMaxSteps:     defaultCallMaxSteps,
				GatewayTimeout:      defaultGwTimeout,
			},
		},
		"some setting set in both env variables and flags": {
			env:       []string{"JUNO_DB_PATH", "/home/env/.juno"},
			inputArgs: []string{"--db-path", "/home/flag/.juno"},
			expectedConfig: &node.Config{
				LogLevel:            defaultLogLevel,
				HTTP:                defaultHTTP,
				HTTPHost:            defaultHost,
				HTTPPort:            defaultHTTPPort,
				Websocket:           defaultWS,
				WebsocketHost:       defaultHost,
				WebsocketPort:       defaultWSPort,
				GRPC:                defaultGRPC,
				GRPCHost:            defaultHost,
				GRPCPort:            defaultGRPCPort,
				Metrics:             defaultMetrics,
				MetricsHost:         defaultHost,
				MetricsPort:         defaultMetricsPort,
				DatabasePath:        "/home/flag/.juno",
				Network:             defaultNetwork,
				Pprof:               defaultPprof,
				PprofHost:           defaultHost,
				PprofPort:           defaultPprofPort,
				Colour:              defaultColour,
				PendingPollInterval: defaultPendingPollInterval,
				MaxVMs:              defaultMaxVMs,
				MaxVMQueue:          2 * defaultMaxVMs,
				RPCMaxBlockScan:     defaultRPCMaxBlockScan,
				DBCacheSize:         defaultMaxCacheSize,
				DBMaxHandles:        defaultMaxHandles,
				RPCCallMaxSteps:     defaultCallMaxSteps,
				GatewayTimeout:      defaultGwTimeout,
			},
		},
		"some setting set in both env variables and config file": {
			cfgFileContents: `db-path: /home/file/.juno`,
			env:             []string{"JUNO_DB_PATH", "/home/env/.juno", "JUNO_GW_API_KEY", "apikey"},
			expectedConfig: &node.Config{
				LogLevel:            defaultLogLevel,
				HTTP:                defaultHTTP,
				HTTPHost:            defaultHost,
				HTTPPort:            defaultHTTPPort,
				Websocket:           defaultWS,
				WebsocketHost:       defaultHost,
				WebsocketPort:       defaultWSPort,
				GRPC:                defaultGRPC,
				GRPCHost:            defaultHost,
				GRPCPort:            defaultGRPCPort,
				Metrics:             defaultMetrics,
				MetricsHost:         defaultHost,
				MetricsPort:         defaultMetricsPort,
				DatabasePath:        "/home/env/.juno",
				Network:             defaultNetwork,
				Pprof:               defaultPprof,
				PprofHost:           defaultHost,
				PprofPort:           defaultPprofPort,
				Colour:              defaultColour,
				PendingPollInterval: defaultPendingPollInterval,
				MaxVMs:              defaultMaxVMs,
				MaxVMQueue:          2 * defaultMaxVMs,
				RPCMaxBlockScan:     defaultRPCMaxBlockScan,
				DBCacheSize:         defaultMaxCacheSize,
				GatewayAPIKey:       "apikey",
				DBMaxHandles:        defaultMaxHandles,
				RPCCallMaxSteps:     defaultCallMaxSteps,
				GatewayTimeout:      defaultGwTimeout,
			},
		},
	}

	junoEnv := unsetJunoPrefixedEnv(t)

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.cfgFile {
				fileN := tempCfgFile(t, tc.cfgFileContents)
				tc.inputArgs = append(tc.inputArgs, "--config", fileN)
			}

			require.True(t, len(tc.env)%2 == 0, "The number of env variables should be an even number")

			if len(tc.env) > 0 {
				for i := 0; i < len(tc.env)/2; i++ {
					require.NoError(t, os.Setenv(tc.env[2*i], tc.env[2*i+1]))
				}
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
			if len(tc.env) > 0 {
				for i := 0; i < len(tc.env)/2; i++ {
					require.NoError(t, os.Unsetenv(tc.env[2*i]))
				}
			}
		})
	}
	setJunoPrefixedEnv(t, junoEnv)
}

func TestGenP2PKeyPair(t *testing.T) {
	cmd := juno.GenP2PKeyPair()
	require.NoError(t, cmd.Execute())
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

func unsetJunoPrefixedEnv(t *testing.T) map[string]string {
	t.Helper()

	const prefix = "JUNO_"
	junoEnv := make(map[string]string)

	for _, e := range os.Environ() {
		pair := strings.Split(e, "=")
		k, v := pair[0], pair[1]

		if strings.HasPrefix(k, prefix) {
			junoEnv[k] = v

			require.NoError(t, os.Unsetenv(k))
		}
	}
	return junoEnv
}

func setJunoPrefixedEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for k, v := range env {
		require.NoError(t, os.Setenv(k, v))
	}
}
