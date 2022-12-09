package main

import (
	"fmt"
	"os"

	"github.com/NethermindEth/juno/node"
	"github.com/NethermindEth/juno/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const greeting = `
       _                    
      | |                   
      | |_   _ _ __   ___   
  _   | | | | | '_ \ / _ \  
 | |__| | |_| | | | | (_) |  
  \____/ \__,_|_| |_|\___/  

Juno is a Go implementation of a StarkNet full node client made with ❤️ by Nethermind.

`

const (
	configF    = "config"
	verbosityF = "verbosity"
	rpcPortF   = "rpc-port"
	metricsF   = "metrics"
	dbPathF    = "db-path"
	networkF   = "network"
	ethNodeF   = "eth-node"

	defaultConfig    = ""
	defaultVerbosity = "info"
	defaultRpcPort   = uint16(6060)
	defaultMetrics   = false
	defaultDbPath    = ""
	defaultNetwork   = utils.GOERLI
	defaultEthNode   = ""

	configFlagUsage    = "The yaml configuration file."
	verbosityFlagUsage = "Verbosity of the logs. Options: debug, info, warn, error, dpanic, " +
		"panic, fatal."
	rpcPortUsage = "The port on which the RPC server will listen for requests. " +
		"Warning: this exposes the node to external requests and potentially DoS attacks."
	metricsUsage = "Enables the metrics server and listens on port 9090."
	dbPathUsage  = "Location of the database files."
	networkUsage = "Available StarkNet networks. Options: 0 = goerli and 1 = mainnet"
	ethNodeUsage = "The Ethereum endpoint to synchronise with. " +
		"If unset feeder gateway will be used."
)

var (
	StarkNetNode node.StarkNetNode
	cfgFile      string
)

func NewCmd(newNodeFn node.NewStarkNetNodeFn, quit <-chan os.Signal) *cobra.Command {
	junoCmd := &cobra.Command{
		Use:   "juno [flags]",
		Short: "StarkNet client implementation in Go.",
	}

	junoCmd.Flags().StringVar(&cfgFile, configF, defaultConfig, configFlagUsage)
	junoCmd.Flags().String(verbosityF, defaultVerbosity, verbosityFlagUsage)
	junoCmd.Flags().Uint16(rpcPortF, defaultRpcPort, rpcPortUsage)
	junoCmd.Flags().Bool(metricsF, defaultMetrics, metricsUsage)
	junoCmd.Flags().String(dbPathF, defaultDbPath, dbPathUsage)
	junoCmd.Flags().Uint8(networkF, uint8(defaultNetwork), networkUsage)
	junoCmd.Flags().String(ethNodeF, defaultEthNode, ethNodeUsage)

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

		StarkNetNode, err = newNodeFn(junoCfg)
		if err != nil {
			return err
		}

		shutDownErrCh := make(chan error)
		go func() {
			<-quit
			if shutdownErr := StarkNetNode.Shutdown(); shutdownErr != nil {
				shutDownErrCh <- shutdownErr
			}
			close(shutDownErrCh)
		}()

		if err = StarkNetNode.Run(); err != nil {
			return err
		}

		if err = <-shutDownErrCh; err != nil {
			return err
		}
		return nil
	}

	return junoCmd
}
