package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	_ "github.com/NethermindEth/juno/jemalloc"
	"github.com/NethermindEth/juno/node"
	"github.com/NethermindEth/juno/p2p"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/automaxprocs/maxprocs"
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
	configF                 = "config"
	logLevelF               = "log-level"
	httpF                   = "http"
	httpHostF               = "http-host"
	httpPortF               = "http-port"
	wsF                     = "ws"
	wsHostF                 = "ws-host"
	wsPortF                 = "ws-port"
	dbPathF                 = "db-path"
	networkF                = "network"
	ethNodeF                = "eth-node"
	pprofF                  = "pprof"
	pprofHostF              = "pprof-host"
	pprofPortF              = "pprof-port"
	colourF                 = "colour"
	pendingPollIntervalF    = "pending-poll-interval"
	p2pF                    = "p2p"
	p2pAddrF                = "p2p-addr"
	p2pPeersF               = "p2p-peers"
	p2pFeederNodeF          = "p2p-feeder-node"
	p2pPrivateKey           = "p2p-private-key"
	metricsF                = "metrics"
	metricsHostF            = "metrics-host"
	metricsPortF            = "metrics-port"
	grpcF                   = "grpc"
	grpcHostF               = "grpc-host"
	grpcPortF               = "grpc-port"
	maxVMsF                 = "max-vms"
	maxVMQueueF             = "max-vm-queue"
	remoteDBF               = "remote-db"
	rpcMaxBlockScanF        = "rpc-max-block-scan"
	dbCacheSizeF            = "db-cache-size"
	dbMaxHandlesF           = "db-max-handles"
	gwAPIKeyF               = "gw-api-key" //nolint: gosec
	gwTimeoutF              = "gw-timeout" //nolint: gosec
	cnNameF                 = "cn-name"
	cnFeederURLF            = "cn-feeder-url"
	cnGatewayURLF           = "cn-gateway-url"
	cnL1ChainIDF            = "cn-l1-chain-id"
	cnL2ChainIDF            = "cn-l2-chain-id"
	cnCoreContractAddressF  = "cn-core-contract-address"
	cnUnverifiableRangeF    = "cn-unverifiable-range"
	callMaxStepsF           = "rpc-call-max-steps"
	corsEnableF             = "rpc-cors-enable"
	versionedConstantsFileF = "versioned-constants-file"

	defaultConfig                   = ""
	defaulHost                      = "localhost"
	defaultHTTP                     = false
	defaultHTTPPort                 = 6060
	defaultWS                       = false
	defaultWSPort                   = 6061
	defaultEthNode                  = ""
	defaultPprof                    = false
	defaultPprofPort                = 6062
	defaultColour                   = true
	defaultPendingPollInterval      = 5 * time.Second
	defaultP2p                      = false
	defaultP2pAddr                  = ""
	defaultP2pPeers                 = ""
	defaultP2pFeederNode            = false
	defaultP2pPrivateKey            = ""
	defaultMetrics                  = false
	defaultMetricsPort              = 9090
	defaultGRPC                     = false
	defaultGRPCPort                 = 6064
	defaultRemoteDB                 = ""
	defaultRPCMaxBlockScan          = math.MaxUint
	defaultCacheSizeMb              = 8
	defaultMaxHandles               = 1024
	defaultGwAPIKey                 = ""
	defaultCNName                   = ""
	defaultCNFeederURL              = ""
	defaultCNGatewayURL             = ""
	defaultCNL1ChainID              = ""
	defaultCNL2ChainID              = ""
	defaultCNCoreContractAddressStr = ""
	defaultCallMaxSteps             = 4_000_000
	defaultGwTimeout                = 5 * time.Second
	defaultCorsEnable               = false
	defaultVersionedConstantsFile   = ""

	configFlagUsage                       = "The YAML configuration file."
	logLevelFlagUsage                     = "Options: trace, debug, info, warn, error."
	httpUsage                             = "Enables the HTTP RPC server on the default port and interface."
	httpHostUsage                         = "The interface on which the HTTP RPC server will listen for requests."
	httpPortUsage                         = "The port on which the HTTP server will listen for requests."
	wsUsage                               = "Enables the WebSocket RPC server on the default port."
	wsHostUsage                           = "The interface on which the WebSocket RPC server will listen for requests."
	wsPortUsage                           = "The port on which the WebSocket server will listen for requests."
	dbPathUsage                           = "Location of the database files."
	networkUsage                          = "Options: mainnet, sepolia, sepolia-integration."
	networkCustomName                     = "Custom network name."
	networkCustomFeederUsage              = "Custom network feeder URL."
	networkCustomGatewayUsage             = "Custom network gateway URL."
	networkCustomL1ChainIDUsage           = "Custom network L1 chain id."
	networkCustomL2ChainIDUsage           = "Custom network L2 chain id."
	networkCustomCoreContractAddressUsage = "Custom network core contract address."
	networkCustomUnverifiableRange        = "Custom network range of blocks to skip hash verifications (e.g. `0,100`)."
	pprofUsage                            = "Enables the pprof endpoint on the default port."
	pprofHostUsage                        = "The interface on which the pprof HTTP server will listen for requests."
	pprofPortUsage                        = "The port on which the pprof HTTP server will listen for requests."
	colourUsage                           = "Use `--colour=false` command to disable colourized outputs (ANSI Escape Codes)."
	ethNodeUsage                          = "WebSocket endpoint of the Ethereum node. To verify the correctness of the L2 chain, " +
		"Juno must connect to an Ethereum node and parse events in the Starknet contract."
	pendingPollIntervalUsage = "Sets how frequently pending block will be updated (0s will disable fetching of pending block)."
	p2pUsage                 = "EXPERIMENTAL: Enables p2p server."
	p2pAddrUsage             = "EXPERIMENTAL: Specify p2p source address as multiaddr."
	p2pPeersUsage            = "EXPERIMENTAL: Specify list of p2p peers split by a comma. " +
		"These peers can be either Feeder or regular nodes."
	p2pFeederNodeUsage = "EXPERIMENTAL: Run juno as a feeder node which will only sync from feeder gateway and gossip the new" +
		" blocks to the network."
	p2pPrivateKeyUsage   = "EXPERIMENTAL: Hexadecimal representation of a private key on the Ed25519 elliptic curve."
	metricsUsage         = "Enables the Prometheus metrics endpoint on the default port."
	metricsHostUsage     = "The interface on which the Prometheus endpoint will listen for requests."
	metricsPortUsage     = "The port on which the Prometheus endpoint will listen for requests."
	grpcUsage            = "Enable the HTTP gRPC server on the default port."
	grpcHostUsage        = "The interface on which the gRPC server will listen for requests."
	grpcPortUsage        = "The port on which the gRPC server will listen for requests."
	maxVMsUsage          = "Maximum number for VM instances to be used for RPC calls concurrently"
	maxVMQueueUsage      = "Maximum number for requests to queue after reaching max-vms before starting to reject incoming requests"
	remoteDBUsage        = "gRPC URL of a remote Juno node"
	rpcMaxBlockScanUsage = "Maximum number of blocks scanned in single starknet_getEvents call"
	dbCacheSizeUsage     = "Determines the amount of memory (in megabytes) allocated for caching data in the database."
	dbMaxHandlesUsage    = "A soft limit on the number of open files that can be used by the DB"
	gwAPIKeyUsage        = "API key for gateway endpoints to avoid throttling" //nolint: gosec
	gwTimeoutUsage       = "Timeout for requests made to the gateway"          //nolint: gosec
	callMaxStepsUsage    = "Maximum number of steps to be executed in starknet_call requests. " +
		"The upper limit is 4 million steps, and any higher value will still be capped at 4 million."
	corsEnableUsage             = "Enable CORS on RPC endpoints"
	versionedConstantsFileUsage = "Use custom versioned constants from provided file"
)

var Version string

func main() {
	if _, err := maxprocs.Set(); err != nil {
		fmt.Printf("error: set maxprocs: %v", err)
		return
	}

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
		Use:     "juno",
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
			return nil //nolint:nilerr
		}

		// TextUnmarshallerHookFunc allows us to unmarshal values that satisfy the
		// encoding.TextUnmarshaller interface (see the LogLevel type for an example).
		if err := v.Unmarshal(config, viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
			mapstructure.TextUnmarshallerHookFunc(), mapstructure.StringToTimeDurationHookFunc()))); err != nil {
			return err
		}

		// Set custom network
		if v.IsSet(cnNameF) {
			l1ChainID, ok := new(big.Int).SetString(v.GetString(cnL1ChainIDF), 0)
			if !ok {
				return fmt.Errorf("invalid %s id %s", cnL1ChainIDF, v.GetString(cnL1ChainIDF))
			}
			unverifRange := v.GetIntSlice(cnUnverifiableRangeF)
			if len(unverifRange) != 2 || unverifRange[0] < 0 || unverifRange[1] < 0 {
				return fmt.Errorf("invalid %s:%v, must be uint array of length 2 (e.g. `0,100`)", cnUnverifiableRangeF, unverifRange)
			}

			config.Network = utils.Network{
				Name:                v.GetString(cnNameF),
				FeederURL:           v.GetString(cnFeederURLF),
				GatewayURL:          v.GetString(cnGatewayURLF),
				L1ChainID:           l1ChainID,
				L2ChainID:           v.GetString(cnL2ChainIDF),
				CoreContractAddress: common.HexToAddress(v.GetString(cnCoreContractAddressF)),
				BlockHashMetaInfo: &utils.BlockHashMetaInfo{
					First07Block:      0,
					UnverifiableRange: []uint64{uint64(unverifRange[0]), uint64(unverifRange[1])},
				},
			}
		}

		return nil
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
	defaultCNUnverifiableRange := []int{} // Uint64Slice is not supported in Flags()

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
	junoCmd.Flags().String(cnNameF, defaultCNName, networkCustomName)
	junoCmd.Flags().String(cnFeederURLF, defaultCNFeederURL, networkCustomFeederUsage)
	junoCmd.Flags().String(cnGatewayURLF, defaultCNGatewayURL, networkCustomGatewayUsage)
	junoCmd.Flags().String(cnL1ChainIDF, defaultCNL1ChainID, networkCustomL1ChainIDUsage)
	junoCmd.Flags().String(cnL2ChainIDF, defaultCNL2ChainID, networkCustomL2ChainIDUsage)
	junoCmd.Flags().String(cnCoreContractAddressF, defaultCNCoreContractAddressStr, networkCustomCoreContractAddressUsage)
	junoCmd.Flags().IntSlice(cnUnverifiableRangeF, defaultCNUnverifiableRange, networkCustomUnverifiableRange)
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
	junoCmd.MarkFlagsRequiredTogether(cnNameF, cnFeederURLF, cnGatewayURLF, cnL1ChainIDF, cnL2ChainIDF, cnCoreContractAddressF, cnUnverifiableRangeF) //nolint:lll
	junoCmd.MarkFlagsMutuallyExclusive(networkF, cnNameF)
	junoCmd.Flags().Uint(callMaxStepsF, defaultCallMaxSteps, callMaxStepsUsage)
	junoCmd.Flags().Duration(gwTimeoutF, defaultGwTimeout, gwTimeoutUsage)
	junoCmd.Flags().Bool(corsEnableF, defaultCorsEnable, corsEnableUsage)
	junoCmd.Flags().String(versionedConstantsFileF, defaultVersionedConstantsFile, versionedConstantsFileUsage)
	junoCmd.MarkFlagsMutuallyExclusive(p2pFeederNodeF, p2pPeersF)
	junoCmd.AddCommand(GenP2PKeyPair())

	return junoCmd
}

func GenP2PKeyPair() *cobra.Command {
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
