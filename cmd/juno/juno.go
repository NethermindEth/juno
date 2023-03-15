package main

import (
	"github.com/NethermindEth/juno/node"
	"github.com/NethermindEth/juno/utils"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var Version string

const (
	configF    = "config"
	verbosityF = "verbosity"
	rpcPortF   = "rpc-port"
	metricsF   = "metrics"
	dbPathF    = "db-path"
	networkF   = "network"
	ethNodeF   = "eth-node"
	pprofF     = "pprof"

	defaultConfig  = ""
	defaultRpcPort = uint16(6060)
	defaultMetrics = false
	defaultDbPath  = ""
	defaultNetwork = utils.MAINNET
	defaultEthNode = ""
	defaultPprof   = false

	configFlagUsage    = "The yaml configuration file."
	verbosityFlagUsage = "Verbosity of the logs. Options: debug, info, warn, error."
	rpcPortUsage       = "The port on which the RPC server will listen for requests. " +
		"Warning: this exposes the node to external requests and potentially DoS attacks."
	metricsUsage = "Enables the metrics server and listens on port 9090."
	dbPathUsage  = "Location of the database files."
	networkUsage = `Available Starknet networks. Options:
0 = mainnet
1 = goerli
2 = goerli2
3 = integration`
	ethNodeUsage = "The Ethereum endpoint to synchronise with. " +
		"If unset feeder gateway will be used."
	pprofUsage = "Enables the pprof server and listens on port 9080."
)

// NewCmd returns a command that can be exected with any of the Cobra Execute* functions.
// The RunE field is set to the user-provided run function, allowing for robust testing setups.
//
//  1. NewCmd is called with a non-nil config and a run function.
//  2. An Execute* function is called on the command returned from step 1.
//  3. The config struct is populated.
//  4. Cobra calls the run function.
func NewCmd(config *node.Config, run func(*cobra.Command, []string) error) *cobra.Command {
	junoCmd := &cobra.Command{
		Use:     "juno [flags]",
		Short:   "Starknet client implementation in Go.",
		Version: Version,
		RunE:    run,
	}

	var cfgFile string

	// PreRunE populates the configuration struct from the Cobra flags and Viper configuration.
	// This is called in step 3 of the process described above.
	junoCmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		v := viper.New()
		if cfgFile != "" {
			v.SetConfigType("yaml")
			v.SetConfigFile(cfgFile)
			if err := v.ReadInConfig(); err != nil {
				return err
			}
		}

		if err := v.BindPFlags(cmd.Flags()); err != nil {
			return nil
		}

		// TextUnmarshallerHookFunc allows us to unmarshal values that satisfy the
		// encoding.TextUnmarshaller interface (see the LogLevel type for an example).
		return v.Unmarshal(config, viper.DecodeHook(mapstructure.TextUnmarshallerHookFunc()))
	}

	junoCmd.Flags().StringVar(&cfgFile, configF, defaultConfig, configFlagUsage)
	// For testing purposes, this variable cannot be declared outside the function because Cobra
	// mutates the value.
	defaultVerbosity := utils.INFO
	junoCmd.Flags().Var(&defaultVerbosity, verbosityF, verbosityFlagUsage)
	junoCmd.Flags().Uint16(rpcPortF, defaultRpcPort, rpcPortUsage)
	junoCmd.Flags().Bool(metricsF, defaultMetrics, metricsUsage)
	junoCmd.Flags().String(dbPathF, defaultDbPath, dbPathUsage)
	junoCmd.Flags().Uint8(networkF, uint8(defaultNetwork), networkUsage)
	junoCmd.Flags().String(ethNodeF, defaultEthNode, ethNodeUsage)
	junoCmd.Flags().Bool(pprofF, defaultPprof, pprofUsage)

	return junoCmd
}
