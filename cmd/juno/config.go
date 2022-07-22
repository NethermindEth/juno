package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"

	"github.com/urfave/cli/v2"
)

// This file defines Juno's top-level configuration. All
// configurations are namespaced by their configuration subsection
// (environment variables are also namespaced with the `JUNO_` prefix
// to prevent clashing with existing variables). This helps keep the UI
// consistent for end users.
//
// For example, to set the log level, users can choose from specifying
// the `--log-level` CLI flag, `JUNO_LOG_LEVEL` environment variable, or
// `log: level: ...` in a yaml configuration file.

// To add a configuration parameter, one only needs to modify this file.
//
// 1. Add the param to the JunoConfig struct (or a relevant embedded
//    struct).
// 2. Define a corresponding flag.
//    - the flag should have an associated environment variable.
//    - the usage section should be highly descriptive.
//    - wrap the flag with `registerFlag` so Juno can recognize it at
//      runtime.
// 3. Process the config parameter in the setConfigs function. This
//    ensures that it is set on the `config` struct at runtime.

// logConfig represents the logger configuration
type logConfig struct {
	Level      string `yaml:"level" mapstructure:"level"`
	EnableJson bool   `yaml:"enable_json" mapstructure:"enable_json"`
}

// rpcConfig represents the juno RPC configuration.
type rpcConfig struct {
	Enable bool `yaml:"enable" mapstructure:"enable"`
	Port   uint `yaml:"port" mapstructure:"port"`
}

// metricsConfig represents the Prometheus Metrics configuration.
type metricsConfig struct {
	Enable bool `yaml:"enable" mapstructure:"enable"`
	Port   uint `yaml:"port" mapstructure:"port"`
}

// ethConfig represents the juno Ethereum configuration.
type ethereumConfig struct {
	Node string `yaml:"node" mapstructure:"node"`
}

// restConfig represents the juno REST configuration.
type restConfig struct {
	Enable bool   `yaml:"enable" mapstructure:"enable"`
	Port   int    `yaml:"port" mapstructure:"port"`
	Prefix string `yaml:"prefix" mapstructure:"prefix"`
}

// starknetConfig represents the juno StarkNet configuration.
type starknetConfig struct {
	Enable         bool   `yaml:"enable" mapstructure:"enable"`
	FeederEndpoint string `yaml:"feeder_endpoint" mapstructure:"feeder_endpoint"`
	Network        string `yaml:"network" mapstructure:"network"`
	SyncFeeder     bool   `yaml:"sync_feeder" mapstructure:"sync_feeder"`
}

// JunoConfig is the top-level juno configuration.
type JunoConfig struct {
	Log               logConfig      `yaml:"logger" mapstructure:"logger"`
	Ethereum          ethereumConfig `yaml:"ethereum" mapstructure:"ethereum"`
	RPC               rpcConfig      `yaml:"rpc" mapstructure:"rpc"`
	Metrics           metricsConfig  `yaml:"metrics" mapstructure:"metrics"`
	REST              restConfig     `yaml:"rest" mapstructure:"rest"`
	DatabaseDirectory string         `yaml:"database_directory" mapstructure:"database_directory"`
	Starknet          starknetConfig `yaml:"starknet" mapstructure:"starknet"`
}

// Flags
var (
	// Logging
	logLevelFlag = registerFlag(&cli.StringFlag{
		Name:  "log-level",
		Value: "info",
		// TODO are these the actual levels that zap uses?
		Usage:   "the log verbosity level. In order of increasing verbosity: debug, dpanic, info, warn, error.",
		EnvVars: []string{"JUNO_LOG_LEVEL"},
	}).(*cli.StringFlag)
	logJsonFlag = registerFlag(&cli.BoolFlag{
		Name:    "log-json",
		Usage:   "print logs in json format. Useful for automated processing. Typically omitted if logs are only viewed from console.",
		EnvVars: []string{"JUNO_LOG_JSON"},
	}).(*cli.BoolFlag)

	// RPC
	rpcEnableFlag = registerFlag(&cli.BoolFlag{
		Name:    "rpc-enable",
		Usage:   "enable the RPC server.", // TODO include security warning?
		EnvVars: []string{"JUNO_RPC_ENABLE"},
	}).(*cli.BoolFlag)
	rpcPortFlag = registerFlag(&cli.UintFlag{
		Name:    "rpc-port",
		Value:   8080,
		Usage:   "specify the port on which the RPC server will listen for requests.",
		EnvVars: []string{"JUNO_RPC_PORT"},
	}).(*cli.UintFlag)

	// Metrics
	metricsEnableFlag = registerFlag(&cli.BoolFlag{
		Name:    "metrics-enable",
		Usage:   "enable the metrics server.",
		EnvVars: []string{"JUNO_METRICS_ENABLE"},
	}).(*cli.BoolFlag)
	metricsPortFlag = registerFlag(&cli.BoolFlag{
		Name:    "metrics-port",
		Usage:   "the port on which the metrics server listens.", // TODO is this correct?
		EnvVars: []string{"JUNO_METRICS_PORT"},
	}).(*cli.BoolFlag)

	// Top-level
	databaseDirectoryFlag = registerFlag(&cli.StringFlag{
		Name:    "database-directory",
		Value:   ".", // Default to current directory
		Usage:   "directory where the database will be stored.",
		EnvVars: []string{"JUNO_DATABASE_DIRECTORY"},
	}).(*cli.StringFlag)

	// Ethereum
	ethereumNodeFlag = registerFlag(&cli.StringFlag{
		Name:    "ethereum-node",
		Usage:   "the endpoint to the ethereum node (e.g. goerli.infura....)", // TODO find better example
		EnvVars: []string{"JUNO_ETHEREUM_NODE"},
	}).(*cli.StringFlag)

	// REST API
	restEnableFlag = registerFlag(&cli.BoolFlag{
		Name:    "rest-enable",
		Usage:   "enable the REST server.", // TODO include security warning?
		EnvVars: []string{"JUNO_REST_ENABLE"},
	}).(*cli.BoolFlag)
	restPortFlag = registerFlag(&cli.UintFlag{
		Name:    "rest-port",
		Value:   8100,
		Usage:   "specify the port on which the rest server will listen for requests.",
		EnvVars: []string{"JUNO_REST_PORT"},
	}).(*cli.UintFlag)
	restPrefixFlag = registerFlag(&cli.StringFlag{
		Name:    "rest-prefix",
		Value:   "/feeder_gateway",
		Usage:   "the url prefix that rest clients should use when sending requests to the rest server",
		EnvVars: []string{"JUNO_REST_PREFIX"},
	}).(*cli.StringFlag)

	// StarkNet
	starknetEnableFlag = registerFlag(&cli.BoolFlag{
		Name:    "starknet-enable",
		Usage:   "if set, the node will synchronize with the StarkNet chain.",
		EnvVars: []string{"JUNO_STARKNET_ENABLE"},
	}).(*cli.BoolFlag)
	starknetFeederEndpointFlag = registerFlag(&cli.StringFlag{
		Name:    "starknet-feeder-endpoint",
		Usage:   "specifies the sequencer endpoint. Mostly useful for those wishing to cache feeder gateway responses in a proxy.",
		EnvVars: []string{"JUNO_STARKNET_FEEDER_ENDPOINT"},
	}).(*cli.StringFlag)
	starknetNetworkFlag = registerFlag(&cli.StringFlag{
		Name:    "starknet-network",
		Usage:   "the StarkNet network with which to sync. Options: mainnet, goerli",
		EnvVars: []string{"JUNO_STARKNET_NETWORK"},
	}).(*cli.StringFlag)
	starknetSyncFeederFlag = registerFlag(&cli.StringFlag{
		Name:    "starknet-sync-feeder",
		Usage:   "sync purely against the feeder gateway, not against L1. Only set to if an Ethereum node is not available. The node provides no guarantees about the integrity of the state when syncing against the feeder api.",
		EnvVars: []string{"JUNO_STARKNET_SYNC_FEEDER"},
	}).(*cli.StringFlag)

	// Obviously, it is the only flag that cannot also be specified in the
	// config file, so it is omitted from the JunoConfig struct.
	configFileFlag = registerFlag(&cli.StringFlag{
		Name:    "config-file",
		Usage:   "path to configuration file.",
		EnvVars: []string{"JUNO_CONFIG_FILE"},
	}).(*cli.StringFlag)
)

// setupConfig processes multiple configuration sources into a single
// struct, representing the definitive configuration for the system.
//
// Conflicts are resolved in order of precendence:
//
// 1. CLI flag
// 2. Environment variable
// 3. Config file
// 4. Default
func setupConfig(ctx *cli.Context) (*JunoConfig, error) {
	config := &JunoConfig{}

	// We load the configs in reverse order of precedence.

	// Load all defaults
	setConfigs(ctx, config, func(_ string) bool { return true })

	// Load config file
	if ctx.IsSet(configFileFlag.Name) {
		configFile, err := os.ReadFile(ctx.String(configFileFlag.Name))
		if err != nil {
			return nil, fmt.Errorf("reading config file: %w", err)
		}
		if err = yaml.Unmarshal(configFile, &config); err != nil { // TODO configure to fail if it doesn't marshal everything perfectly
			return nil, fmt.Errorf("decoding yaml config file: %w", err)
		}
	}

	// Load CLI args and environment vars (precedence is handled by
	// `ctx`). Only change the value if the user specified the
	// param.
	setConfigs(ctx, config, func(name string) bool { return ctx.IsSet(name) })

	return config, nil
}

// setConfigs initializes `config` based on the state of `ctx`.
// A config param is set if `shouldBeSet` is true.
func setConfigs(ctx *cli.Context, config *JunoConfig, shouldBeSet func(name string) bool) error {
	setConfigVal := func(val any, name string) {
		if shouldBeSet(name) {
			switch val.(type) {
			case *bool:
				val = ctx.Bool(name)
			case *uint:
				val = ctx.Uint(name)
			case *string:
				val = ctx.String(name)
			}
		}
	}

	// Logging
	setConfigVal(&config.Log.Level, logLevelFlag.Name)
	setConfigVal(&config.Log.EnableJson, logJsonFlag.Name)

	// RPC
	setConfigVal(&config.RPC.Enable, rpcEnableFlag.Name)
	setConfigVal(&config.RPC.Port, rpcPortFlag.Name)

	// Metrics
	setConfigVal(&config.Metrics.Enable, metricsEnableFlag.Name)
	setConfigVal(&config.Metrics.Port, metricsPortFlag.Name)

	// Ethereum
	setConfigVal(&config.Ethereum.Node, ethereumNodeFlag.Name)

	// REST API
	setConfigVal(&config.REST.Enable, restEnableFlag.Name)
	setConfigVal(&config.REST.Port, restPortFlag.Name)
	setConfigVal(&config.REST.Prefix, restPrefixFlag.Name)

	// StarkNet
	setConfigVal(&config.Starknet.Enable, starknetEnableFlag.Name)
	setConfigVal(&config.Starknet.FeederEndpoint, starknetFeederEndpointFlag.Name)
	setConfigVal(&config.Starknet.Network, starknetNetworkFlag.Name)
	setConfigVal(&config.Starknet.SyncFeeder, starknetSyncFeederFlag.Name)

	// Database directory
	setConfigVal(&config.DatabaseDirectory, databaseDirectoryFlag.Name)
	if _, err := os.Stat(config.DatabaseDirectory); err != nil {
		return fmt.Errorf("database directory: %w", err)
	}

	return nil
}

var flags []cli.Flag

func registerFlag(flag cli.Flag) cli.Flag {
	flags = append(flags, flag)
	return flag
}
