package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/NethermindEth/juno/node"
	"github.com/NethermindEth/juno/p2p"
	"github.com/NethermindEth/juno/utils"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	_ "go.uber.org/automaxprocs"
)

const greeting = `
       _                    
      | |                   
      | |_   _ _ __   ___   
  _   | | | | | '_ \ / _ \  
 | |__| | |_| | | | | (_) |  
  \____/ \__,_|_| |_|\___/ %s

Juno is a Go implementation of a Starknet full-node client created by Nethermind.

`

const (
	configF              = "config"
	logLevelF            = "log-level"
	httpF                = "http"
	httpHostF            = "http-host"
	httpPortF            = "http-port"
	wsF                  = "ws"
	wsHostF              = "ws-host"
	wsPortF              = "ws-port"
	dbPathF              = "db-path"
	networkF             = "network"
	ethNodeF             = "eth-node"
	pprofF               = "pprof"
	pprofHostF           = "pprof-host"
	pprofPortF           = "pprof-port"
	colourF              = "colour"
	pendingPollIntervalF = "pending-poll-interval"
	p2pF                 = "p2p"
	p2pAddrF             = "p2p-addr"
	p2pPeersF            = "p2p-peers"
	p2pFeederNodeF       = "p2p-feeder-node"
	p2pPrivateKey        = "p2p-private-key"
	metricsF             = "metrics"
	metricsHostF         = "metrics-host"
	metricsPortF         = "metrics-port"
	grpcF                = "grpc"
	grpcHostF            = "grpc-host"
	grpcPortF            = "grpc-port"
	maxVMsF              = "max-vms"
	maxVMQueueF          = "max-vm-queue"
	remoteDBF            = "remote-db"
	rpcMaxBlockScanF     = "rpc-max-block-scan"
	dbCacheSizeF         = "db-cache-size"
	dbMaxHandlesF        = "db-max-handles"
	gwAPIKeyF            = "gw-api-key" //nolint: gosec

	defaultConfig              = ""
	defaulHost                 = "localhost"
	defaultHTTP                = false
	defaultHTTPPort            = 6060
	defaultWS                  = false
	defaultWSPort              = 6061
	defaultEthNode             = ""
	defaultPprof               = false
	defaultPprofPort           = 6062
	defaultColour              = true
	defaultPendingPollInterval = time.Duration(0)
	defaultP2p                 = false
	defaultP2pAddr             = ""
	defaultP2pPeers            = ""
	defaultP2pFeederNode       = false
	defaultP2pPrivateKey       = ""
	defaultMetrics             = false
	defaultMetricsPort         = 9090
	defaultGRPC                = false
	defaultGRPCPort            = 6064
	defaultRemoteDB            = ""
	defaultRPCMaxBlockScan     = math.MaxUint
	defaultCacheSizeMb         = 8
	defaultMaxHandles          = 1024
	defaultGwAPIKey            = ""

	configFlagUsage   = "The yaml configuration file."
	logLevelFlagUsage = "Options: debug, info, warn, error."
	httpUsage         = "Enables the HTTP RPC server on the default port and interface."
	httpHostUsage     = "The interface on which the HTTP RPC server will listen for requests."
	httpPortUsage     = "The port on which the HTTP server will listen for requests."
	wsUsage           = "Enables the Websocket RPC server on the default port."
	wsHostUsage       = "The interface on which the Websocket RPC server will listen for requests."
	wsPortUsage       = "The port on which the websocket server will listen for requests."
	dbPathUsage       = "Location of the database files."
	networkUsage      = "Options: mainnet, goerli, goerli2, integration, sepolia, sepolia-integration."
	pprofUsage        = "Enables the pprof endpoint on the default port."
	pprofHostUsage    = "The interface on which the pprof HTTP server will listen for requests."
	pprofPortUsage    = "The port on which the pprof HTTP server will listen for requests."
	colourUsage       = "Uses --colour=false command to disable colourized outputs (ANSI Escape Codes)."
	ethNodeUsage      = "Websocket endpoint of the Ethereum node. In order to verify the correctness of the L2 chain, " +
		"Juno must connect to an Ethereum node and parse events in the Starknet contract."
	pendingPollIntervalUsage = "Sets how frequently pending block will be updated (disabled by default)."
	p2pUsage                 = "Enables p2p server."
	p2pAddrUsage             = "Specify p2p source address as multiaddr."
	p2pPeersUsage            = "Specify list of p2p peers split by a comma. These peers can be either Feeder or regular nodes."
	p2pFeederNodeUsage       = "Run juno as a feeder node which will only sync from feeder gateway and gossip the new blocks to the network."
	p2pPrivateKeyUsage       = "Hexadecimal representation of a private key on the Ed25519 elliptic curve."
	metricsUsage             = "Enables the prometheus metrics endpoint on the default port."
	metricsHostUsage         = "The interface on which the prometheus endpoint will listen for requests."
	metricsPortUsage         = "The port on which the prometheus endpoint will listen for requests."
	grpcUsage                = "Enable the HTTP GRPC server on the default port."
	grpcHostUsage            = "The interface on which the GRPC server will listen for requests."
	grpcPortUsage            = "The port on which the GRPC server will listen for requests."
	maxVMsUsage              = "Maximum number for VM instances to be used for RPC calls concurrently"
	maxVMQueueUsage          = "Maximum number for requests to queue after reaching max-vms before starting to reject incoming requets"
	remoteDBUsage            = "gRPC URL of a remote Juno node"
	rpcMaxBlockScanUsage     = "Maximum number of blocks scanned in single starknet_getEvents call"
	dbCacheSizeUsage         = "Determines the amount of memory (in megabytes) allocated for caching data in the database."
	dbMaxHandlesUsage        = "A soft limit on the number of open files that can be used by the DB"
	gwAPIKeyUsage            = "API key for gateway endpoints to avoid throttling" //nolint: gosec
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
		fmt.Printf(greeting, Version)

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
//
//nolint:funlen
func NewCmd(config *node.Config, run func(*cobra.Command, []string) error) *cobra.Command {
	junoCmd := &cobra.Command{
		Use:     "juno [flags]",
		Short:   "Starknet client implementation in Go.",
		Version: Version,
		RunE:    run,
	}

	var cfgFile string
	var cwdErr error

	// PreRunE populates the configuration struct from the Cobra flags and Viper configuration.
	// This is called in step 3 of the process described above.
	junoCmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		// If we couldn't find the current working directory and the database path is empty,
		// return the error.
		if cwdErr != nil && config.DatabasePath == "" {
			return fmt.Errorf("find current working directory: %v", cwdErr)
		}

		v := viper.New()
		if cfgFile != "" {
			v.SetConfigType("yaml")
			v.SetConfigFile(cfgFile)
			if err := v.ReadInConfig(); err != nil {
				return err
			}
		}

		v.AutomaticEnv()
		v.SetEnvPrefix("JUNO")
		v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
		if err := v.BindPFlags(cmd.Flags()); err != nil {
			return nil
		}

		// TextUnmarshallerHookFunc allows us to unmarshal values that satisfy the
		// encoding.TextUnmarshaller interface (see the LogLevel type for an example).
		return v.Unmarshal(config, viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
			mapstructure.TextUnmarshallerHookFunc(), mapstructure.StringToTimeDurationHookFunc())))
	}

	var defaultDBPath string
	defaultDBPath, cwdErr = os.Getwd()
	// Use empty string if we can't get the working directory.
	// We don't want to return an error here since that would make `--help` fail.
	// If the error is non-nil and a db path is not provided by the user, we'll return it in PreRunE.
	if cwdErr == nil {
		defaultDBPath = filepath.Join(defaultDBPath, "juno")
	}

	// For testing purposes, these variables cannot be declared outside the function because Cobra
	// may mutate their values.
	defaultLogLevel := utils.INFO
	defaultNetwork := utils.Mainnet
	defaultMaxVMs := 3 * runtime.GOMAXPROCS(0)

	junoCmd.Flags().StringVar(&cfgFile, configF, defaultConfig, configFlagUsage)
	junoCmd.Flags().Var(&defaultLogLevel, logLevelF, logLevelFlagUsage)
	junoCmd.Flags().Bool(httpF, defaultHTTP, httpUsage)
	junoCmd.Flags().String(httpHostF, defaulHost, httpHostUsage)
	junoCmd.Flags().Uint16(httpPortF, defaultHTTPPort, httpPortUsage)
	junoCmd.Flags().Bool(wsF, defaultWS, wsUsage)
	junoCmd.Flags().String(wsHostF, defaulHost, wsHostUsage)
	junoCmd.Flags().Uint16(wsPortF, defaultWSPort, wsPortUsage)
	junoCmd.Flags().String(dbPathF, defaultDBPath, dbPathUsage)
	junoCmd.Flags().Var(&defaultNetwork, networkF, networkUsage)
	junoCmd.Flags().String(ethNodeF, defaultEthNode, ethNodeUsage)
	junoCmd.Flags().Bool(pprofF, defaultPprof, pprofUsage)
	junoCmd.Flags().String(pprofHostF, defaulHost, pprofHostUsage)
	junoCmd.Flags().Uint16(pprofPortF, defaultPprofPort, pprofPortUsage)
	junoCmd.Flags().Bool(colourF, defaultColour, colourUsage)
	junoCmd.Flags().Duration(pendingPollIntervalF, defaultPendingPollInterval, pendingPollIntervalUsage)
	junoCmd.Flags().Bool(p2pF, defaultP2p, p2pUsage)
	junoCmd.Flags().String(p2pAddrF, defaultP2pAddr, p2pAddrUsage)
	junoCmd.Flags().String(p2pPeersF, defaultP2pPeers, p2pPeersUsage)
	junoCmd.Flags().Bool(p2pFeederNodeF, defaultP2pFeederNode, p2pFeederNodeUsage)
	junoCmd.Flags().String(p2pPrivateKey, defaultP2pPrivateKey, p2pPrivateKeyUsage)
	junoCmd.Flags().Bool(metricsF, defaultMetrics, metricsUsage)
	junoCmd.Flags().String(metricsHostF, defaulHost, metricsHostUsage)
	junoCmd.Flags().Uint16(metricsPortF, defaultMetricsPort, metricsPortUsage)
	junoCmd.Flags().Bool(grpcF, defaultGRPC, grpcUsage)
	junoCmd.Flags().String(grpcHostF, defaulHost, grpcHostUsage)
	junoCmd.Flags().Uint16(grpcPortF, defaultGRPCPort, grpcPortUsage)
	junoCmd.Flags().Uint(maxVMsF, uint(defaultMaxVMs), maxVMsUsage)
	junoCmd.Flags().Uint(maxVMQueueF, 2*uint(defaultMaxVMs), maxVMQueueUsage)
	junoCmd.Flags().String(remoteDBF, defaultRemoteDB, remoteDBUsage)
	junoCmd.Flags().Uint(rpcMaxBlockScanF, defaultRPCMaxBlockScan, rpcMaxBlockScanUsage)
	junoCmd.Flags().Uint(dbCacheSizeF, defaultCacheSizeMb, dbCacheSizeUsage)
	junoCmd.Flags().String(gwAPIKeyF, defaultGwAPIKey, gwAPIKeyUsage)
	junoCmd.Flags().Int(dbMaxHandlesF, defaultMaxHandles, dbMaxHandlesUsage)

	junoCmd.MarkFlagsMutuallyExclusive(p2pFeederNodeF, p2pPeersF)
	junoCmd.AddCommand(NewCmdGenP2PKeyPair())

	return junoCmd
}

func NewCmdGenP2PKeyPair() *cobra.Command {
	return &cobra.Command{
		Use:   "genp2pkeypair",
		Short: "Generate private key pair for p2p.",
		RunE: func(*cobra.Command, []string) error {
			priv, pub, id, err := p2p.GenKeyPair()
			if err != nil {
				return err
			}

			rawPriv, err := priv.Raw()
			if err != nil {
				return err
			}

			privHex := make([]byte, hex.EncodedLen(len(rawPriv)))
			hex.Encode(privHex, rawPriv)
			fmt.Println("P2P Private Key:", string(privHex))

			rawPub, err := pub.Raw()
			if err != nil {
				return err
			}

			pubHex := make([]byte, hex.EncodedLen(len(rawPub)))
			hex.Encode(pubHex, rawPub)
			fmt.Println("P2P Public Key:", string(pubHex))

			fmt.Println("P2P PeerID:", id)
			return nil
		},
	}
}
