package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/NethermindEth/juno/node"
	"github.com/NethermindEth/juno/utils"
	"github.com/mitchellh/mapstructure"
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

Juno is a Go implementation of a Starknet full node client created by Nethermind.`

const (
	configF              = "config"
	logLevelF            = "log-level"
	rpcPortF             = "rpc-port"
	grpcPortF            = "grpc-port"
	dbPathF              = "db-path"
	networkF             = "network"
	ethNodeF             = "eth-node"
	pprofF               = "pprof"
	colourF              = "colour"
	pendingPollIntervalF = "pending-poll-interval"
	p2pF                 = "p2p"
	p2pAddrF             = "p2p-addr"
	p2pBootPeersF        = "p2p-boot-peers"
	metricsF             = "metrics"
	metricsPortF         = "metrics-port"

	defaultConfig              = ""
	defaultRPCPort             = 6060
	defaultGRPCPort            = 0
	defaultDBPath              = ""
	defaultEthNode             = ""
	defaultPprof               = false
	defaultColour              = true
	defaultPendingPollInterval = time.Duration(0)
	defaultP2p                 = false
	defaultP2pAddr             = ""
	defaultP2pBootPeers        = ""
	defaultMetrics             = false
	defaultMetricsPort         = 9090

	configFlagUsage   = "The yaml configuration file."
	logLevelFlagUsage = "Options: debug, info, warn, error."
	rpcPortUsage      = "The port on which the RPC server will listen for requests."
	grpcPortUsage     = "The port on which the gRPC server will listen for requests."
	dbPathUsage       = "Location of the database files."
	networkUsage      = "Options: mainnet, goerli, goerli2, integration."
	pprofUsage        = "Enables the pprof server and listens on port 9080."
	colourUsage       = "Uses --colour=false command to disable colourized outputs (ANSI Escape Codes)."
	ethNodeUsage      = "Websocket endpoint of the Ethereum node. In order to verify the correctness of the L2 chain, " +
		"Juno must connect to an Ethereum node and parse events in the Starknet contract."
	pendingPollIntervalUsage = "Sets how frequently pending block will be updated (disabled by default)"
	p2pUsage                 = "enable p2p server"
	p2PAddrUsage             = "specify p2p source address as multiaddr"
	p2pBootPeersUsage        = "specify list of p2p boot peers splitted by a comma"
	metricsUsage             = "enable prometheus endpoint"
	metricsPortUsage         = "The port on which the prometheus server will listen for requests"
)

var Version string

func main() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-quit
		cancel()
	}()

	config := new(node.Config)
	cmd := NewCmd(config, func(cmd *cobra.Command, _ []string) error {
		fmt.Printf("%s\n\n", greeting)

		n, err := node.New(config, Version)
		if err != nil {
			return err
		}

		n.Run(cmd.Context())
		return nil
	})

	if err := cmd.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}

// NewCmd returns a command that can be executed with any of the Cobra Execute* functions.
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
		return v.Unmarshal(config, viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
			mapstructure.TextUnmarshallerHookFunc(), mapstructure.StringToTimeDurationHookFunc())))
	}

	// For testing purposes, these variables cannot be declared outside the function because Cobra
	// may mutate their values.
	defaultLogLevel := utils.INFO
	defaultNetwork := utils.MAINNET

	junoCmd.Flags().StringVar(&cfgFile, configF, defaultConfig, configFlagUsage)
	junoCmd.Flags().Var(&defaultLogLevel, logLevelF, logLevelFlagUsage)
	junoCmd.Flags().Uint16(rpcPortF, defaultRPCPort, rpcPortUsage)
	junoCmd.Flags().Uint16(grpcPortF, defaultGRPCPort, grpcPortUsage)
	junoCmd.Flags().String(dbPathF, defaultDBPath, dbPathUsage)
	junoCmd.Flags().Var(&defaultNetwork, networkF, networkUsage)
	junoCmd.Flags().String(ethNodeF, defaultEthNode, ethNodeUsage)
	junoCmd.Flags().Bool(pprofF, defaultPprof, pprofUsage)
	junoCmd.Flags().Bool(colourF, defaultColour, colourUsage)
	junoCmd.Flags().Duration(pendingPollIntervalF, defaultPendingPollInterval, pendingPollIntervalUsage)
	junoCmd.Flags().Bool(p2pF, defaultP2p, p2pUsage)
	junoCmd.Flags().String(p2pAddrF, defaultP2pAddr, p2PAddrUsage)
	junoCmd.Flags().String(p2pBootPeersF, defaultP2pBootPeers, p2pBootPeersUsage)
	junoCmd.Flags().Bool(metricsF, defaultMetrics, metricsUsage)
	junoCmd.Flags().Uint16(metricsPortF, defaultMetricsPort, metricsPortUsage)

	return junoCmd
}
