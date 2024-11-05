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

type Flag struct {
	name         string
	usage        string
	defaultValue any
}

var dbPath = Flag{name: "db-path", usage: "Location of the database files.", defaultValue: ""}

//nolint:lll
var ethFlag = Flag{name: "eth-node", usage: "WebSocket endpoint of the Ethereum node. To verify the correctness of the L2 chain, Juno must connect to an Ethereum node and parse events in the Starknet contract.", defaultValue: ""}

//nolint:lll
var disableL1Verification = Flag{name: "disable-l1-verification", usage: "Disables L1 verification since an Ethereum node is not provided.", defaultValue: false}
var network = Flag{name: "network", usage: "Options: mainnet, sepolia, sepolia-integration.", defaultValue: utils.Mainnet}

//nolint:lll
var (
	p2pFeederNode = Flag{name: "p2p-feeder-node", usage: "EXPERIMENTAL: Run juno as a feeder node which will only sync from feeder gateway and gossip the new blocks to the network.", defaultValue: false}
	p2pPeers      = Flag{name: "p2p-peers", usage: "EXPERIMENTAL: Specify list of p2p peers split by a comma. These peers can be either Feeder or regular nodes.", defaultValue: ""}
)

//nolint:lll
var (
	cnName                = Flag{name: "cn-name", usage: "Custom network name.", defaultValue: ""}
	cnFeederURL           = Flag{name: "cn-feeder-url", usage: "Custom network feeder URL.", defaultValue: ""}
	cnGatewayURL          = Flag{name: "cn-gateway-url", usage: "Custom network gateway URL.", defaultValue: ""}
	cnL1ChainID           = Flag{name: "cn-l1-chain-id", usage: "Custom network L1 chain id.", defaultValue: ""}
	cnL2ChainID           = Flag{name: "cn-l2-chain-id", usage: "Custom network L2 chain id.", defaultValue: ""}
	cnCoreContractAddress = Flag{name: "cn-core-contract-address", usage: "Custom network core contract address.", defaultValue: ""}
	cnUnverifiableRange   = Flag{name: "cn-unverifiable-range", usage: "Custom network range of blocks to skip hash verifications (e.g. `0,100`).", defaultValue: []int{}}
)

//nolint:lll,mnd
var flags = []Flag{
	dbPath,
	ethFlag,
	disableL1Verification,
	network,
	// p2p flags
	p2pFeederNode,
	p2pPeers,
	// network flags
	cnName,
	cnFeederURL,
	cnGatewayURL,
	cnL1ChainID,
	cnL2ChainID,
	{name: "config", usage: "The YAML configuration file.", defaultValue: ""},
	{name: "log-level", usage: "Options: trace, debug, info, warn, error.", defaultValue: utils.INFO},
	{name: "http", usage: "Enables the HTTP RPC server on the default port and interface.", defaultValue: false},
	{name: "http-host", usage: "The interface on which the HTTP RPC server will listen for requests.", defaultValue: "localhost"},
	{name: "http-port", usage: "The port on which the HTTP server will listen for requests.", defaultValue: uint16(6060)},
	{name: "ws", usage: "Enables the WebSocket RPC server on the default port.", defaultValue: false},
	{name: "ws-host", usage: "The interface on which the WebSocket RPC server will listen for requests.", defaultValue: "localhost"},
	{name: "ws-port", usage: "The port on which the WebSocket server will listen for requests.", defaultValue: uint16(6061)},
	{name: "pprof", usage: "Enables the pprof endpoint on the default port.", defaultValue: false},
	{name: "pprof-host", usage: "The interface on which the pprof HTTP server will listen for requests.", defaultValue: "localhost"},
	{name: "pprof-port", usage: "The port on which the pprof HTTP server will listen for requests.", defaultValue: uint16(6062)},
	{name: "colour", usage: "Use `--colour=false` command to disable colourized outputs (ANSI Escape Codes).", defaultValue: true},
	{name: "pending-poll-interval", usage: "Sets how frequently pending block will be updated (0s will disable fetching of pending block).", defaultValue: 5 * time.Second},
	{name: "p2p", usage: "EXPERIMENTAL: Enables p2p server.", defaultValue: false},
	{name: "p2p-addr", usage: "EXPERIMENTAL: Specify p2p listening source address as multiaddr. Example: /ip4/0.0.0.0/tcp/7777", defaultValue: ""},
	{name: "p2p-public-addr", usage: "EXPERIMENTAL: Specify p2p public address as multiaddr. Example: /ip4/35.243.XXX.XXX/tcp/7777", defaultValue: ""},
	{name: "p2p-private-key", usage: "EXPERIMENTAL: Hexadecimal representation of a private key on the Ed25519 elliptic curve.", defaultValue: ""},
	{name: "metrics", usage: "Enables the Prometheus metrics endpoint on the default port.", defaultValue: false},
	{name: "metrics-host", usage: "The interface on which the Prometheus endpoint will listen for requests.", defaultValue: "localhost"},
	{name: "metrics-port", usage: "The port on which the Prometheus endpoint will listen for requests.", defaultValue: uint16(9090)},
	{name: "grpc", usage: "Enable the HTTP gRPC server on the default port.", defaultValue: false},
	{name: "grpc-host", usage: "The interface on which the gRPC server will listen for requests.", defaultValue: "localhost"},
	{name: "grpc-port", usage: "The port on which the gRPC server will listen for requests.", defaultValue: uint16(6064)},
	{name: "max-vms", usage: "Maximum number for VM instances to be used for RPC calls concurrently", defaultValue: uint(3 * runtime.GOMAXPROCS(0))},
	{name: "max-vm-queue", usage: "Maximum number for requests to queue after reaching max-vms before starting to reject incoming requests", defaultValue: uint(50)},
	{name: "remote-db", usage: "gRPC URL of a remote Juno node", defaultValue: ""},
	{name: "rpc-max-block-scan", usage: "Maximum number of blocks scanned in single starknet_getEvents call", defaultValue: uint(math.MaxUint)},
	{name: "db-cache-size", usage: "Determines the amount of memory (in megabytes) allocated for caching data in the database.", defaultValue: uint(1024)},
	{name: "db-max-handles", usage: "A soft limit on the number of open files that can be used by the DB", defaultValue: int(1024)},
	{name: "gw-api-key", usage: "API key for gateway endpoints to avoid throttling", defaultValue: ""},
	{name: "gw-timeout", usage: "Timeout for requests made to the gateway", defaultValue: 5 * time.Second},
	{name: "rpc-call-max-steps", usage: "Maximum number of steps to be executed in starknet_call requests. The upper limit is 4 million steps, and any higher value will still be capped at 4 million.", defaultValue: uint(4000000)},
	{name: "rpc-cors-enable", usage: "Enable CORS on RPC endpoints", defaultValue: false},
	{name: "versioned-constants-file", usage: "Use custom versioned constants from provided file", defaultValue: ""},
	{name: "plugin-path", usage: "Path to the plugin .so file", defaultValue: ""},
}

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
		if v.IsSet(cnName.name) {
			l1ChainID, ok := new(big.Int).SetString(v.GetString(cnL1ChainID.name), 0)
			if !ok {
				return fmt.Errorf("invalid %s id %s", cnL1ChainID.name, v.GetString(cnL1ChainID.name))
			}
			unverifRange := v.GetIntSlice(cnUnverifiableRange.name)
			if len(unverifRange) != 2 || unverifRange[0] < 0 || unverifRange[1] < 0 {
				return fmt.Errorf("invalid %s:%v, must be uint array of length 2 (e.g. `0,100`)", cnUnverifiableRange.name, unverifRange)
			}

			config.Network = utils.Network{
				Name:                v.GetString(cnName.name),
				FeederURL:           v.GetString(cnFeederURL.name),
				GatewayURL:          v.GetString(cnGatewayURL.name),
				L1ChainID:           l1ChainID,
				L2ChainID:           v.GetString(cnL2ChainID.name),
				CoreContractAddress: common.HexToAddress(v.GetString(cnCoreContractAddress.name)),
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
	_ = defaultLogLevel // todo recheck

	cmdFlags := junoCmd.Flags()
	for _, flag := range flags {
		switch defaultValue := flag.defaultValue.(type) {
		case string:
			cmdFlags.String(flag.name, defaultValue, flag.usage)
		case bool:
			cmdFlags.Bool(flag.name, defaultValue, flag.usage)
		case time.Duration:
			cmdFlags.Duration(flag.name, defaultValue, flag.usage)
		case uint16:
			cmdFlags.Uint16(flag.name, defaultValue, flag.usage)
		case uint:
			cmdFlags.Uint(flag.name, defaultValue, flag.usage)
		case int:
			cmdFlags.Int(flag.name, defaultValue, flag.usage)
		case []int:
			junoCmd.Flags().IntSlice(flag.name, defaultValue, flag.usage)
		}
	}
	junoCmd.MarkFlagsMutuallyExclusive(ethFlag.name, disableL1Verification.name)
	junoCmd.MarkFlagsMutuallyExclusive(network.name, cnName.name)
	junoCmd.MarkFlagsMutuallyExclusive(p2pFeederNode.name, p2pPeers.name)
	junoCmd.MarkFlagsRequiredTogether(cnName.name, cnFeederURL.name, cnGatewayURL.name, cnL1ChainID.name, cnL2ChainID.name, cnCoreContractAddress.name, cnUnverifiableRange.name) //nolint:lll

	/*
		todo fix it
		junoCmd.Flags().Var(&defaultLogLevel, logLevelF, logLevelFlagUsage)
		junoCmd.Flags().Var(&defaultNetwork, networkF, networkUsage)
		junoCmd.Flags().StringVar(&cfgFile, configF, defaultConfig, configFlagUsage)
	*/

	junoCmd.AddCommand(GenP2PKeyPair(), DBCmd(defaultDBPath))

	return junoCmd
}
