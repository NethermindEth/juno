package main

// notest
import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/NethermindEth/juno/internal/config"
	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/db/abi"
	"github.com/NethermindEth/juno/internal/db/block"
	"github.com/NethermindEth/juno/internal/db/contracthash"
	"github.com/NethermindEth/juno/internal/db/state"
	"github.com/NethermindEth/juno/internal/db/transaction"
	. "github.com/NethermindEth/juno/internal/log"
	metric "github.com/NethermindEth/juno/internal/metrics/prometheus"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/NethermindEth/juno/pkg/rest"
	"github.com/NethermindEth/juno/pkg/rpc"
	"github.com/NethermindEth/juno/pkg/starknet"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/torquem-ch/mdbx-go/mdbx"
)

var (
	rpcServer     *rpc.Server
	metricsServer *metric.Server
	restServer    *rest.Server

	stateSynchronizer *starknet.Synchronizer

	contractHashManager *contracthash.Manager
	abiManager          *abi.Manager
	stateManager        *state.Manager
	transactionManager  *transaction.Manager
	blockManager        *block.Manager
)

func main() {
	fmt.Printf(`
       _                    
      | |                   
      | |_   _ _ __   ___   
  _   | | | | | '_ \ / _ \  
 | |__| | |_| | | | | (_) |  
  \____/ \__,_|_| |_|\___/  

Juno is a Go implementation of a StarkNet full node client made with ❤️ by Nethermind.

`)

	// Start juno
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	cfg := &config.Juno{}
	var configFile string

	rootCmd := &cobra.Command{
		Use:   "juno [options]",
		Short: "StarkNet client implementation in Go.",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return loadConfig(cmd, &configFile)
		},
		Run: func(_ *cobra.Command, _ []string) {
			juno(cfg)
		},
	}

	// Commands
	rootCmd.AddCommand(newConfigureCmd(cfg, &configFile))

	// Flags

	if configDir, err := os.UserConfigDir(); err != nil {
		configFile = ""
	} else {
		configFile = filepath.Join(configDir, "juno", "juno.yaml")
	}
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", configFile, "the yaml configuration file.")

	// Log
	rootCmd.PersistentFlags().StringVar(&cfg.Log.Level, "log-level", "info", "verbosity of the logs. Options: debug, info, warn, error, dpanic, panic, fatal.")
	rootCmd.PersistentFlags().BoolVar(&cfg.Log.Json, "log-json", false, "print logs in json format. Useful for automated processing. Typically omitted if logs are only viewed from console.")

	// RPC
	rootCmd.PersistentFlags().BoolVar(&cfg.Rpc.Enable, "rpc-enable", false, "enable the RPC server. Warning: this exposes the node to external requests and potentially DoS attacks.")
	rootCmd.PersistentFlags().UintVar(&cfg.Rpc.Port, "rpc-port", 8080, "the port on which the RPC server will listen for requests.")

	// Metrics
	rootCmd.PersistentFlags().BoolVar(&cfg.Metrics.Enable, "metrics-enable", false, "enable the metrics server.")
	rootCmd.PersistentFlags().UintVar(&cfg.Metrics.Port, "metrics-port", 9090, "the port on which the metrics server listens.")

	// REST
	rootCmd.PersistentFlags().BoolVar(&cfg.Rest.Enable, "rest-enable", false, "enable the REST server.")
	rootCmd.PersistentFlags().UintVar(&cfg.Rest.Port, "rest-port", 8100, "the port on which the rest server will listen. Warning: this exposes the node to external requests and potentially DoS attacks.")
	rootCmd.PersistentFlags().StringVar(&cfg.Rest.Prefix, "rest-prefix", "/feeder_gateway", "part of the the url endpoint for the REST server.")

	// Database
	cfg.Database.Path, _ = config.UserDataDir() // Empty string is a suitable default if there is an error
	rootCmd.PersistentFlags().StringVar(&cfg.Database.Path, "database-path", cfg.Database.Path, "location of the database files.")

	// StarkNet
	rootCmd.PersistentFlags().BoolVar(&cfg.Starknet.Enable, "starknet-enable", false, "if set, the node will synchronize with the StarkNet chain.")
	rootCmd.PersistentFlags().StringVar(&cfg.Starknet.Sequencer, "starknet-sequencer", "https://alpha-mainnet.starknet.io", "the sequencer endpoint. Useful for those wishing to cache feeder gateway responses in a proxy.")
	rootCmd.PersistentFlags().StringVar(&cfg.Starknet.Network, "starknet-network", "mainnet", "the StarkNet network with which to sync. Options: mainnet, goerli")
	rootCmd.PersistentFlags().BoolVar(&cfg.Starknet.ApiSync, "starknet-apisync", false, "sync with the feeder gateway, not against L1. Only set if an Ethereum node is not available. The node provides no guarantees about the integrity of the state when syncing with the feeder gateway.")
	rootCmd.PersistentFlags().StringVar(&cfg.Starknet.EthNode, "starknet-ethnode", "", "the endpoint to the ethereum node. Required if one wants to maintain the state in a truly trustless way.")

	return rootCmd
}

type keyToEnvReplacer struct{}

// Replace is used to conviently convert from a viper key to an
// environment variable. E.g. "database-path" --> "JUNO_DATABASE_PATH"
//
// It is public to fulfill the viper.EnvKeyReplacer interface.
// Not meant for external use.
func (r *keyToEnvReplacer) Replace(key string) string {
	return "JUNO_" + strings.ToUpper(strings.ReplaceAll(key, "-", "_"))
}

// Loads the viper configuration and handles precedence in the following order:
//
// 1. CLI params
// 2. Environment variables
// 3. Config file
// 4. Defaults
//
// Inspired by https://github.com/carolynvs/stingoftheviper/blob/e0d04fd2334bdf677a7f8825404a70e3c2c7e7d0/main.go
func loadConfig(cmd *cobra.Command, configFile *string) error {
	keyDelimiter := "-"
	v := viper.NewWithOptions(viper.KeyDelimiter(keyDelimiter), viper.EnvKeyReplacer(&keyToEnvReplacer{}))

	// The `--config` flag is unique. We need to evaluate its
	// argument/env var before loading the configuration file.
	setFlagValue(cmd, v, cmd.Flag("config"))

	v.AddConfigPath(filepath.Dir(*configFile))
	v.SetConfigType("yaml")
	v.SetConfigName(filepath.Base(*configFile))

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
		fmt.Printf("config file not found at %s: falling back to CLI params, environment vars, and defaults\n", *configFile)
	}

	cmd.Flags().VisitAll(func(f *pflag.Flag) { setFlagValue(cmd, v, f) })

	return nil
}

// setFlagValue binds each cobra flag to its associated viper
// configuration (config file and environment variable)
func setFlagValue(cmd *cobra.Command, v *viper.Viper, f *pflag.Flag) {
	// Bind flag to environment variable
	v.BindEnv(f.Name)

	// Apply the viper config value to the flag when the flag is not
	// set and viper has a value
	if !f.Changed && v.IsSet(f.Name) {
		val := v.Get(f.Name)
		cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val))
	}
}

// juno is the main entrypoint for the Juno node.
func juno(cfg *config.Juno) {
	fmt.Printf("using config: %+v\n\n", cfg)

	// Configure the logger first so we can use it
	setupLogger(&cfg.Log)
	setupInterruptHandler(cfg)
	mdbxEnv := setupMdbxEnv(&cfg.Database)
	setupDatabase(mdbxEnv)
	feederClient := setupFeederGateway(&cfg.Starknet)
	rpcErrChan := make(chan error)
	setupRpc(&cfg.Rpc, feederClient, rpcErrChan)
	metricsErrChan := make(chan error)
	setupMetrics(&cfg.Metrics, metricsErrChan)
	restErrChan := make(chan error)
	setupRest(&cfg.Rest, feederClient, restErrChan)
	setupStateSynchronizer(&cfg.Starknet, feederClient, mdbxEnv)

	if !cfg.Starknet.Enable {
		// Currently, Juno is only useful for storing StarkNet
		// state locally. We can't do that if StarkNet syncing
		// is disabled, so we exit with a heplful message
		fmt.Println("StarkNet synchronization is disabled. To enable it, use the --starknet-enable flag.")
		shutdown(cfg)
	} else {
		// Wait until error
		checkErrChs(rpcErrChan, metricsErrChan, restErrChan)
	}
}

func setupLogger(cfg *config.Log) {
	if err := ReplaceGlobalLogger(cfg.Json, cfg.Level); err != nil {
		fmt.Printf("failed to initialize logger: %s\n", err)
		os.Exit(1)
	}
}

func setupMdbxEnv(cfg *config.Database) *mdbx.Env {
	mdbxEnv, err := db.NewMDBXEnv(cfg.Path, 100, 0)
	if err != nil {
		Logger.With("databasePath", cfg.Path, "error", err).Fatal("Failed to initialize database environment")
	}
	return mdbxEnv
}

func setupStateSynchronizer(cfg *config.Starknet, feederClient *feeder.Client, mdbxEnv *mdbx.Env) {
	if cfg.Enable {
		var ethereumClient *ethclient.Client
		if !cfg.ApiSync {
			var err error
			ethereumClient, err = ethclient.Dial(cfg.EthNode)
			if err != nil {
				Logger.With("Error", err).Fatal("Unable to connect to Ethereum Client")
			}
		}
		// Synchronizer for Starknet State
		synchronizerDb, err := db.NewMDBXDatabase(mdbxEnv, "SYNCHRONIZER")
		if err != nil {
			Logger.With("Error", err).Fatal("Error starting the SYNCHRONIZER database")
		}
		stateSynchronizer = starknet.NewSynchronizer(synchronizerDb, ethereumClient, feederClient,
			contractHashManager, abiManager, stateManager, transactionManager, blockManager, cfg.Network)
		if err := stateSynchronizer.UpdateState(cfg.ApiSync); err != nil {
			Logger.Fatal("Failed to start State Synchronizer: ", err.Error())
		}
	}
}

func setupRpc(cfg *config.Rpc, feederClient *feeder.Client, errChan chan error) {
	if cfg.Enable {
		rpcServer = rpc.NewServer(":"+strconv.FormatUint(uint64(cfg.Port), 10), feederClient, abiManager,
			stateManager, transactionManager, blockManager)
		rpcServer.ListenAndServe(errChan)
	}
}

func setupMetrics(cfg *config.Metrics, errChan chan error) {
	if cfg.Enable {
		metricsServer = metric.SetupMetric(":" + strconv.FormatUint(uint64(cfg.Port), 10))
		metricsServer.ListenAndServe(errChan)
	}
}

func setupRest(cfg *config.Rest, feederClient *feeder.Client, errChan chan error) {
	if cfg.Enable {
		restServer = rest.NewServer(":"+strconv.FormatUint(uint64(cfg.Port), 10), feederClient)
		restServer.ListenAndServe(errChan)
	}
}

func setupFeederGateway(cfg *config.Starknet) *feeder.Client {
	return feeder.NewClient(cfg.Sequencer, "/feeder_gateway", nil)
}

func setupDatabase(mdbxEnv *mdbx.Env) {
	var err error
	var dbName string

	if err != nil {
		Logger.Fatal("Failed to create MDBX Database environment: ", err)
	}

	logDBErr := func(name string, err error) {
		if err != nil {
			Logger.Fatalf("Failed to create %s database: %s", name, err)
		}
	}

	dbName = "CONTRACT_HASH"
	contractHashDb, err := db.NewMDBXDatabase(mdbxEnv, dbName)
	logDBErr(dbName, err)

	dbName = "ABI"
	abiDb, err := db.NewMDBXDatabase(mdbxEnv, dbName)
	logDBErr(dbName, err)

	dbName = "CODE"
	codeDb, err := db.NewMDBXDatabase(mdbxEnv, dbName)
	logDBErr(dbName, err)

	dbName = "STORAGE"
	storageDb, err := db.NewMDBXDatabase(mdbxEnv, dbName)
	logDBErr(dbName, err)

	dbName = "TRANSACTION"
	txDb, err := db.NewMDBXDatabase(mdbxEnv, dbName)
	logDBErr(dbName, err)

	dbName = "RECEIPT"
	receiptDb, err := db.NewMDBXDatabase(mdbxEnv, dbName)
	logDBErr(dbName, err)

	dbName = "BLOCK"
	blockDb, err := db.NewMDBXDatabase(mdbxEnv, dbName)
	logDBErr(dbName, err)

	contractHashManager = contracthash.NewManager(contractHashDb)
	abiManager = abi.NewManager(abiDb)
	stateManager = state.NewManager(codeDb, db.NewBlockSpecificDatabase(storageDb))
	transactionManager = transaction.NewManager(txDb, receiptDb)
	blockManager = block.NewManager(blockDb)
}

func setupInterruptHandler(cfg *config.Juno) {
	// Handle signal interrupts and exits.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func(sig chan os.Signal) {
		<-sig
		Logger.Info("Shutting down Juno...")
		shutdown(cfg)
		os.Exit(0)
	}(sig)
}

func shutdown(cfg *config.Juno) {
	contractHashManager.Close()
	abiManager.Close()
	stateManager.Close()
	transactionManager.Close()
	blockManager.Close()
	if cfg.Starknet.Enable {
		stateSynchronizer.Close()
	}

	shutdownTimeout := 5 * time.Second

	if cfg.Rpc.Enable {
		if err := rpcServer.Close(shutdownTimeout); err != nil {
			Logger.Fatal("Failed to shutdown RPC server gracefully: ", err)
		}
	}

	if cfg.Metrics.Enable {
		if err := metricsServer.Close(shutdownTimeout); err != nil {
			Logger.Fatal("Failed to shutdown Metrics server gracefully: ", err)
		}
	}

	if cfg.Rest.Enable {
		if err := restServer.Close(shutdownTimeout); err != nil {
			Logger.Fatal("Failed to shutdown REST server gracefully: ", err)
		}
	}
}

func checkErrChs(errChs ...chan error) {
	for _, errCh := range errChs {
		for {
			if err := <-errCh; err != nil {
				Logger.Fatal(err)
			}
		}
	}
}
