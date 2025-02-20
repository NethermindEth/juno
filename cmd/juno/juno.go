package main

import (
	"context"
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

	_ "github.com/NethermindEth/juno/encoder/registry"
	_ "github.com/NethermindEth/juno/jemalloc"
	"github.com/NethermindEth/juno/node"
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

type empty struct{}

type setting[T string | uint | uint16 | bool | time.Duration | []int | empty | utils.Network] struct {
	flag         string
	defaultValue T
	usage        string
}

const (
	defaulthost = "localhost"
)

var (
	emptyValue             = empty{}
	configFile             = setting[string]{"config", "", "The YAML configuration file."}
	logLevel               = setting[string]{"log-level", utils.INFO.String(), "Options: trace, debug, info, warn, error."}
	http                   = setting[bool]{"http", false, "Enables the HTTP RPC server on the default port and interface."}
	httpHost               = setting[string]{"http-host", defaulthost, "The interface on which the HTTP RPC server will listen for requests."}
	httpPort               = setting[uint16]{"http-port", 6060, "The port on which the HTTP server will listen for requests."}
	ws                     = setting[bool]{"ws", false, "Enables the WebSocket RPC server on the default port."}
	wsHost                 = setting[string]{"ws-host", defaulthost, "The interface on which the WebSocket RPC server will listen for requests."} //nolint:lll
	wsPort                 = setting[uint16]{"ws-port", 6061, "The port on which the WebSocket server will listen for requests."}
	dbPath                 = setting[empty]{"db-path", emptyValue, "Location of the database files."}
	network                = setting[utils.Network]{"network", utils.Mainnet, "Options: mainnet, sepolia, sepolia-integration."}
	ethNode                = setting[string]{"eth-node", "", "WebSocket endpoint of the Ethereum node. To verify the correctness of the L2 chain, Juno must connect to an Ethereum node and parse events in the Starknet contract."} //nolint:lll
	disableL1Verification  = setting[bool]{"disable-l1-verification", false, "Disables L1 verification since an Ethereum node is not provided."}                                                                                     //nolint:lll
	pprof                  = setting[bool]{"pprof", false, "Enables the pprof endpoint on the default port."}
	pprofHost              = setting[string]{"pprof-host", defaulthost, "The interface on which the pprof HTTP server will listen for requests."} //nolint:lll
	pprofPort              = setting[uint16]{"pprof-port", 6062, "The port on which the pprof HTTP server will listen for requests."}
	colour                 = setting[bool]{"colour", true, "Use `--colour=false` command to disable colourized outputs (ANSI Escape Codes)."}
	pendingPollInterval    = setting[time.Duration]{"pending-poll-interval", 5 * time.Second, "Sets how frequently pending block will be updated (0s will disable fetching of pending block)."} //nolint:lll
	p2pIsOn                = setting[bool]{"p2p", false, "EXPERIMENTAL: Enables p2p server."}
	p2pAddr                = setting[string]{"p2p-addr", "", "EXPERIMENTAL: Specify p2p listening source address as multiaddr. Example: /ip4/0.0.0.0/tcp/7777"}                                    //nolint:lll
	p2pPublicAddr          = setting[string]{"p2p-public-addr", "", "EXPERIMENTAL: Specify p2p public address as multiaddr. Example: /ip4/35.243.XXX.XXX/tcp/7777"}                                //nolint:lll
	p2pPeers               = setting[string]{"p2p-peers", "", "EXPERIMENTAL: Specify list of p2p peers split by a comma. These peers can be either Feeder or regular nodes."}                      //nolint:lll
	p2pFeederNode          = setting[bool]{"p2p-feeder-node", false, "EXPERIMENTAL: Run juno as a feeder node which will only sync from feeder gateway and gossip the new blocks to the network."} //nolint:lll
	p2pPrivateKey          = setting[string]{"p2p-private-key", "", "EXPERIMENTAL: Hexadecimal representation of a private key on the Ed25519 elliptic curve."}                                    //nolint:lll
	metrics                = setting[bool]{"metrics", false, "Enables the Prometheus metrics endpoint on the default port."}
	metricsHost            = setting[string]{"metrics-host", defaulthost, "The interface on which the Prometheus endpoint will listen for requests."} //nolint:lll
	metricsPort            = setting[uint16]{"metrics-port", 9090, "The port on which the Prometheus endpoint will listen for requests."}
	grpc                   = setting[bool]{"grpc", false, "Enable the HTTP gRPC server on the default port."}
	grpcHost               = setting[string]{"grpc-host", defaulthost, "The interface on which the gRPC server will listen for requests."}
	grpcPort               = setting[uint16]{"grpc-port", 6064, "The port on which the gRPC server will listen for requests."}
	maxVMs                 = setting[uint]{"max-vms", 0, "Maximum number for VM instances to be used for RPC calls concurrently"}                                        //nolint:lll
	maxVMQueue             = setting[uint]{"max-vm-queue", 0, "Maximum number for requests to queue after reaching max-vms before starting to reject incoming requests"} //nolint:lll
	remoteDB               = setting[string]{"remote-db", "", "gRPC URL of a remote Juno node"}
	rpcMaxBlockScan        = setting[uint]{"rpc-max-block-scan", math.MaxUint, "Maximum number of blocks scanned in single starknet_getEvents call"}            //nolint:lll
	dbCacheSize            = setting[uint]{"db-cache-size", 1024, "Determines the amount of memory (in megabytes) allocated for caching data in the database."} //nolint:lll
	dbMaxHandles           = setting[uint]{"db-max-handles", 1024, "A soft limit on the number of open files that can be used by the DB"}
	gwAPIKey               = setting[string]{"gw-api-key", "", "API key for gateway endpoints to avoid throttling"}
	gwTimeout              = setting[time.Duration]{"gw-timeout", 5 * time.Second, "Timeout for requests made to the gateway"}
	cnName                 = setting[string]{"cn-name", "", "Custom network name."}
	cnFeederURL            = setting[string]{"cn-feeder-url", "", "Custom network feeder URL."}
	cnGatewayURL           = setting[string]{"cn-gateway-url", "", "Custom network gateway URL."}
	cnL1ChainID            = setting[string]{"cn-l1-chain-id", "", "Custom network L1 chain ID."}
	cnL2ChainID            = setting[string]{"cn-l2-chain-id", "", "Custom network L2 chain ID."}
	cnCoreContractAddress  = setting[string]{"cn-core-contract-address", "", "Custom network core contract address."}
	cnUnverifiableRange    = setting[[]int]{"cn-unverifiable-range", []int{}, "Custom network unverifiable range."}
	callMaxSteps           = setting[uint]{"rpc-call-max-steps", 4_000_000, "Maximum number of steps to be executed in starknet_call requests. The upper limit is 4 million steps, and any higher value will still be capped at 4 million."} //nolint:lll
	corsEnable             = setting[bool]{"rpc-cors-enable", false, "Enable CORS on RPC endpoints"}
	versionedConstantsFile = setting[string]{"versioned-constants-file", "", "Use custom versioned constants from provided file"}
	pluginPath             = setting[string]{"plugin-path", "", "Path to the plugin .so file"}
	logHost                = setting[string]{"log-host", defaulthost, "The interface on which the log level HTTP server will listen for requests."} //nolint:lll
	logPort                = setting[uint16]{"log-port", 0, "The port on which the log level HTTP server will listen for requests."}
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
		_, err := fmt.Fprintf(cmd.OutOrStdout(), greeting, Version)
		if err != nil {
			return err
		}

		logLevel := utils.NewLogLevel(utils.INFO)
		err = logLevel.Set(config.LogLevel)
		if err != nil {
			return err
		}

		n, err := node.New(config, Version, logLevel)
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
		if v.IsSet(cnName.flag) {
			l1ChainID, ok := new(big.Int).SetString(v.GetString(cnL1ChainID.flag), 0)
			if !ok {
				return fmt.Errorf("invalid %s id %s", cnL1ChainID.flag, v.GetString(cnL1ChainID.flag))
			}
			unverifRange := v.GetIntSlice(cnUnverifiableRange.flag)
			if len(unverifRange) != 2 || unverifRange[0] < 0 || unverifRange[1] < 0 {
				return fmt.Errorf("invalid %s:%v, must be uint array of length 2 (e.g. `0,100`)", cnUnverifiableRange.flag, unverifRange)
			}

			config.Network = utils.Network{
				Name:                v.GetString(cnName.flag),
				FeederURL:           v.GetString(cnFeederURL.flag),
				GatewayURL:          v.GetString(cnGatewayURL.flag),
				L1ChainID:           l1ChainID,
				L2ChainID:           v.GetString(cnL2ChainID.flag),
				CoreContractAddress: common.HexToAddress(v.GetString(cnCoreContractAddress.flag)),
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
	network.defaultValue = utils.Mainnet
	maxVMs.defaultValue = uint(3 * runtime.GOMAXPROCS(0))
	maxVMQueue.defaultValue = 2 * maxVMs.defaultValue

	junoCmd.Flags().StringVar(&cfgFile, configFile.flag, configFile.defaultValue, configFile.usage)
	junoCmd.Flags().String(logLevel.flag, logLevel.defaultValue, logLevel.usage)
	junoCmd.Flags().Bool(http.flag, http.defaultValue, http.usage)
	junoCmd.Flags().String(httpHost.flag, httpHost.defaultValue, httpHost.usage)
	junoCmd.Flags().Uint16(httpPort.flag, httpPort.defaultValue, httpPort.usage)
	junoCmd.Flags().Bool(ws.flag, ws.defaultValue, ws.usage)
	junoCmd.Flags().String(wsHost.flag, wsHost.defaultValue, wsHost.usage)
	junoCmd.Flags().Uint16(wsPort.flag, wsPort.defaultValue, wsPort.usage)
	junoCmd.Flags().String(dbPath.flag, defaultDBPath, dbPath.usage)
	junoCmd.Flags().Var(&network.defaultValue, network.flag, network.usage)
	junoCmd.Flags().String(cnName.flag, cnName.defaultValue, cnName.usage)
	junoCmd.Flags().String(cnFeederURL.flag, cnFeederURL.defaultValue, cnFeederURL.usage)
	junoCmd.Flags().String(cnGatewayURL.flag, cnGatewayURL.defaultValue, cnGatewayURL.usage)
	junoCmd.Flags().String(cnL1ChainID.flag, cnL1ChainID.defaultValue, cnL1ChainID.usage)
	junoCmd.Flags().String(cnL2ChainID.flag, cnL2ChainID.defaultValue, cnL2ChainID.usage)
	junoCmd.Flags().String(cnCoreContractAddress.flag, cnCoreContractAddress.defaultValue, cnCoreContractAddress.usage)
	junoCmd.Flags().IntSlice(cnUnverifiableRange.flag, cnUnverifiableRange.defaultValue, cnUnverifiableRange.usage)
	junoCmd.Flags().String(ethNode.flag, ethNode.defaultValue, ethNode.usage)
	junoCmd.Flags().Bool(disableL1Verification.flag, disableL1Verification.defaultValue, disableL1Verification.usage)
	junoCmd.MarkFlagsMutuallyExclusive(ethNode.flag, disableL1Verification.flag)
	junoCmd.Flags().Bool(pprof.flag, pprof.defaultValue, pprof.usage)
	junoCmd.Flags().String(pprofHost.flag, pprofHost.defaultValue, pprofHost.usage)
	junoCmd.Flags().Uint16(pprofPort.flag, pprofPort.defaultValue, pprofPort.usage)
	junoCmd.Flags().Bool(colour.flag, colour.defaultValue, colour.usage)
	junoCmd.Flags().Duration(pendingPollInterval.flag, pendingPollInterval.defaultValue, pendingPollInterval.usage)
	junoCmd.Flags().Bool(p2pIsOn.flag, p2pIsOn.defaultValue, p2pIsOn.usage)
	junoCmd.Flags().String(p2pAddr.flag, p2pAddr.defaultValue, p2pAddr.usage)
	junoCmd.Flags().String(p2pPublicAddr.flag, p2pPublicAddr.defaultValue, p2pPublicAddr.usage)
	junoCmd.Flags().String(p2pPeers.flag, p2pPeers.defaultValue, p2pPeers.usage)
	junoCmd.Flags().Bool(p2pFeederNode.flag, p2pFeederNode.defaultValue, p2pFeederNode.usage)
	junoCmd.Flags().String(p2pPrivateKey.flag, p2pPrivateKey.defaultValue, p2pPrivateKey.usage)
	junoCmd.Flags().Bool(metrics.flag, metrics.defaultValue, metrics.usage)
	junoCmd.Flags().String(metricsHost.flag, metricsHost.defaultValue, metricsHost.usage)
	junoCmd.Flags().Uint16(metricsPort.flag, metricsPort.defaultValue, metricsPort.usage)
	junoCmd.Flags().Bool(grpc.flag, grpc.defaultValue, grpc.usage)
	junoCmd.Flags().String(grpcHost.flag, grpcHost.defaultValue, grpcHost.usage)
	junoCmd.Flags().Uint16(grpcPort.flag, grpcPort.defaultValue, grpcPort.usage)
	junoCmd.Flags().Uint(maxVMs.flag, maxVMs.defaultValue, maxVMs.usage)
	junoCmd.Flags().Uint(maxVMQueue.flag, maxVMQueue.defaultValue, maxVMQueue.usage)
	junoCmd.Flags().String(remoteDB.flag, remoteDB.defaultValue, remoteDB.usage)
	junoCmd.Flags().Uint(rpcMaxBlockScan.flag, rpcMaxBlockScan.defaultValue, rpcMaxBlockScan.usage)
	junoCmd.Flags().Uint(dbCacheSize.flag, dbCacheSize.defaultValue, dbCacheSize.usage)
	junoCmd.Flags().String(gwAPIKey.flag, gwAPIKey.defaultValue, gwAPIKey.usage)
	junoCmd.Flags().Int(dbMaxHandles.flag, int(dbMaxHandles.defaultValue), dbMaxHandles.usage)
	junoCmd.MarkFlagsRequiredTogether(
		cnName.flag, cnFeederURL.flag, cnGatewayURL.flag, cnL1ChainID.flag,
		cnL2ChainID.flag, cnCoreContractAddress.flag, cnUnverifiableRange.flag,
	)
	junoCmd.MarkFlagsMutuallyExclusive(network.flag, cnName.flag)
	junoCmd.Flags().Uint(callMaxSteps.flag, callMaxSteps.defaultValue, callMaxSteps.usage)
	junoCmd.Flags().Duration(gwTimeout.flag, gwTimeout.defaultValue, gwTimeout.usage)
	junoCmd.Flags().Bool(corsEnable.flag, corsEnable.defaultValue, corsEnable.usage)
	junoCmd.Flags().String(versionedConstantsFile.flag, versionedConstantsFile.defaultValue, versionedConstantsFile.usage)
	junoCmd.MarkFlagsMutuallyExclusive(p2pFeederNode.flag, p2pPeers.flag)
	junoCmd.Flags().String(pluginPath.flag, pluginPath.defaultValue, pluginPath.usage)
	junoCmd.Flags().String(logHost.flag, logHost.defaultValue, logHost.usage)
	junoCmd.Flags().Uint16(logPort.flag, logPort.defaultValue, logPort.usage)

	junoCmd.AddCommand(GenP2PKeyPair(), DBCmd(defaultDBPath))

	return junoCmd
}
