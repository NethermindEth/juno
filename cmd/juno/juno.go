package main

import (
	"fmt"

	"github.com/NethermindEth/juno/node"
	"github.com/NethermindEth/juno/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var Version string

const greeting = `
       _                    
      | |                   
      | |_   _ _ __   ___   
  _   | | | | | '_ \ / _ \  
 | |__| | |_| | | | | (_) |  
  \____/ \__,_|_| |_|\___/  

Juno is a Go implementation of a Starknet full node client created by Nethermind.

`

const (
	configF    = "config"
	verbosityF = "verbosity"
	rpcPortF   = "rpc-port"
	metricsF   = "metrics"
	dbPathF    = "db-path"
	networkF   = "network"
	ethNodeF   = "eth-node"
	pprofF     = "pprof"

	defaultConfig    = ""
	defaultVerbosity = utils.INFO
	defaultRpcPort   = uint16(6060)
	defaultMetrics   = false
	defaultDbPath    = ""
	defaultNetwork   = utils.MAINNET
	defaultEthNode   = ""
	defaultPprof     = false

	configFlagUsage    = "The yaml configuration file."
	verbosityFlagUsage = `Verbosity of the logs. Options:
0 = debug
1 = info
2 = warn
3 = error
`
	rpcPortUsage = "The port on which the RPC server will listen for requests. " +
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

var (
	StarknetNode node.StarknetNode
	cfgFile      string
)

func NewCmd(newNodeFn node.NewStarknetNodeFn) *cobra.Command {
	junoCmd := &cobra.Command{
		Use:     "juno [flags]",
		Short:   "Starknet client implementation in Go.",
		Version: Version,
	}

	junoCmd.Flags().StringVar(&cfgFile, configF, defaultConfig, configFlagUsage)
	junoCmd.Flags().Uint8(verbosityF, uint8(defaultVerbosity), verbosityFlagUsage)
	junoCmd.Flags().Uint16(rpcPortF, defaultRpcPort, rpcPortUsage)
	junoCmd.Flags().Bool(metricsF, defaultMetrics, metricsUsage)
	junoCmd.Flags().String(dbPathF, defaultDbPath, dbPathUsage)
	junoCmd.Flags().Uint8(networkF, uint8(defaultNetwork), networkUsage)
	junoCmd.Flags().String(ethNodeF, defaultEthNode, ethNodeUsage)
	junoCmd.Flags().Bool(pprofF, defaultPprof, pprofUsage)

	junoCmd.RunE = func(cmd *cobra.Command, _ []string) error {
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

		if _, err := fmt.Fprint(cmd.OutOrStdout(), greeting); err != nil {
			return err
		}

		var err error
		junoCfg := new(node.Config)

		if err = v.Unmarshal(junoCfg); err != nil {
			return err
		}

		StarknetNode, err = newNodeFn(junoCfg)
		if err != nil {
			return err
		}

		StarknetNode.Run(cmd.Context())
		return nil
	}

	return junoCmd
}
