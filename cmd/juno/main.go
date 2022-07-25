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

// TODO eliminate all global variables

const (
	mdbxOptMaxDb uint64 = 100
	mdbxFlags    uint   = 0
)

var (
	mdbxEnv *mdbx.Env

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
	cfg := &config.Config{}
	var configFile string

	// rootCmd is the root command of the application.
	rootCmd := &cobra.Command{
		// TODO add help template with examples
		Use:   "juno [options]", // TODO usage template
		Short: "StarkNet client implementation in Go.",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return loadConfig(cmd, &configFile)
		},
		Run: func(_ *cobra.Command, _ []string) {
			juno(cfg)
		},
	}

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
	rootCmd.PersistentFlags().BoolVar(&cfg.RPC.Enable, "rpc-enable", false, "enable the RPC server. Warning: this exposes the node to external requests and potentially DoS attacks.")
	rootCmd.PersistentFlags().UintVar(&cfg.RPC.Port, "rpc-port", 8080, "the port on which the RPC server will listen for requests.")

	// Metrics
	rootCmd.PersistentFlags().BoolVar(&cfg.Metrics.Enable, "metrics-enable", false, "enable the metrics server.")
	rootCmd.PersistentFlags().UintVar(&cfg.Metrics.Port, "metrics-port", 8081, "the port on which the metrics server listens.")

	// REST
	rootCmd.PersistentFlags().BoolVar(&cfg.REST.Enable, "rest-enable", false, "enable the REST server.")
	rootCmd.PersistentFlags().UintVar(&cfg.REST.Port, "rest-port", 8100, "the port on which the rest server will listen. Warning: this exposes the node to external requests and potentially DoS attacks.")

	// Ethereum
	rootCmd.PersistentFlags().StringVar(&cfg.Ethereum.Node, "ethereum-node", "", "the endpoint to the ethereum node. Required if one wants to maintain the state in a truly trustless way.")

	// Database
	cfg.Database.Path, _ = config.UserDataDir() // Empty string is a suitable default if there is an error
	rootCmd.PersistentFlags().StringVar(&cfg.Database.Path, "database-path", cfg.Database.Path, "location of the database files")
	rootCmd.PersistentFlags().StringVar(&cfg.Database.Name, "database-name", "juno", "name of the database. Useful if maintaining databases from different networks.")

	// StarkNet
	rootCmd.PersistentFlags().BoolVar(&cfg.Starknet.Enable, "starknet-enable", false, "if set, the node will synchronize with the StarkNet chain.")
	rootCmd.PersistentFlags().StringVar(&cfg.Starknet.Sequencer, "starknet-sequencer", "https://alpha-mainnet.starknet.io", "the sequencer endpoint. Useful for those wishing to cache feeder gateway responses in a proxy.")
	rootCmd.PersistentFlags().StringVar(&cfg.Starknet.Network, "starknet-network", "mainnet", "the StarkNet network with which to sync. Options: mainnet, goerli")
	rootCmd.PersistentFlags().BoolVar(&cfg.Starknet.ApiSync, "starknet-apisync", false, "sync with the feeder gateway, not against L1. Only set if an Ethereum node is not available. The node provides no guarantees about the integrity of the state when syncing with the feeder gateway.")

	return rootCmd
}

type keyToEnvReplacer struct{}

func (r *keyToEnvReplacer) Replace(key string) string {
	// E.g. bind --database-path to JUNO_DATABASE_PATH
	return "JUNO_" + strings.ToUpper(strings.ReplaceAll(key, "-", "_"))
}

// Inspired by https://github.com/carolynvs/stingoftheviper/blob/e0d04fd2334bdf677a7f8825404a70e3c2c7e7d0/main.go#L103
func loadConfig(cmd *cobra.Command, configFile *string) error {
	keyDelimiter := "."
	v := viper.NewWithOptions(viper.KeyDelimiter(keyDelimiter), viper.EnvKeyReplacer(&keyToEnvReplacer{}))

	// The `--config` flag is an oddball. We evaluate its
	// argument before loading the configuration file.
	setFlagValue(cmd, v, cmd.Flag("config"))

	v.AddConfigPath(filepath.Dir(*configFile))
	v.SetConfigType("yaml")
	v.SetConfigName(filepath.Base(*configFile))

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
		fmt.Printf("config file not found at %s: falling back to CLI params, environment vars, and defaults\n", *configFile)
	}

	cmd.Flags().VisitAll(func(f *pflag.Flag) { setFlagValue(cmd, v, f) })

	return nil
}

// Bind each cobra flag to its associated viper configuration (config file and environment variable)
func setFlagValue(cmd *cobra.Command, v *viper.Viper, f *pflag.Flag) {
	v.BindEnv(f.Name)

	// Apply the viper config value to the flag when the flag is not set and viper has a value
	if !f.Changed && v.IsSet(f.Name) {
		val := v.Get(f.Name)
		cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val))
	}
}

func juno(cfg *config.Config) {
	fmt.Printf("using config: %+v\n", cfg)

	setupLogger(cfg)
	setupInterruptHandler(cfg)
	setupDatabaseManagers(cfg)
	feederClient := setupFeederGateway(cfg)
	errChs := []chan error{make(chan error), make(chan error), make(chan error)}
	setupServers(cfg, feederClient, errChs[0], errChs[1], errChs[2])
	setupStateSynchronizer(cfg, feederClient)

	checkErrChs(errChs)
}

func setupLogger(cfg *config.Config) {
	if err := ReplaceGlobalLogger(cfg.Log.Json, cfg.Log.Level); err != nil {
		fmt.Printf("failed to initialize logger: %s\n", err)
		os.Exit(1)
	}
}

func setupStateSynchronizer(cfg *config.Config, feederClient *feeder.Client) {
	if cfg.Starknet.Enable {
		var ethereumClient *ethclient.Client
		if !cfg.Starknet.ApiSync {
			var err error
			ethereumClient, err = ethclient.Dial(cfg.Ethereum.Node)
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
			contractHashManager, abiManager, stateManager, transactionManager, blockManager, cfg.Starknet.Network)
		if err := stateSynchronizer.UpdateState(cfg.Starknet.ApiSync); err != nil {
			Logger.Fatal("Failed to start State Synchronizer: ", err.Error())
		}
	}
}

func setupServers(cfg *config.Config, feederClient *feeder.Client, rpcErrChan, metricsErrChan, restErrChan chan error) {
	if cfg.RPC.Enable {
		rpcServer = rpc.NewServer(":"+strconv.FormatUint(uint64(cfg.RPC.Port), 10), feederClient, abiManager,
			stateManager, transactionManager, blockManager)
		rpcServer.ListenAndServe(rpcErrChan)
	}

	if cfg.Metrics.Enable {
		metricsServer = metric.SetupMetric(":" + strconv.FormatUint(uint64(cfg.Metrics.Port), 10))
		metricsServer.ListenAndServe(metricsErrChan)
	}

	if cfg.REST.Enable {
		restServer = rest.NewServer(":"+strconv.FormatUint(uint64(cfg.REST.Port), 10),
			cfg.Starknet.Sequencer, cfg.REST.Prefix)
		restServer.ListenAndServe(restErrChan)
	}
}

func setupFeederGateway(cfg *config.Config) *feeder.Client {
	return feeder.NewClient(cfg.Starknet.Sequencer, "/feeder_gateway", nil)
}

func setupDatabaseManagers(cfg *config.Config) {
	var err error
	var dbName string

	mdbxEnv, err = db.NewMDBXEnv(cfg.Database.Path, mdbxOptMaxDb, mdbxFlags)
	if err != nil {
		Logger.Fatal("Failed to create MDBX Database environment: ", err)
	}

	logDBErr := func(name string, err error) {
		if err != nil {
			Logger.Fatalf("Failed to create %s database: %s", name, err.Error())
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

func setupInterruptHandler(cfg *config.Config) {
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

func shutdown(cfg *config.Config) {
	contractHashManager.Close()
	abiManager.Close()
	stateManager.Close()
	transactionManager.Close()
	blockManager.Close()
	if cfg.Starknet.Enable {
		stateSynchronizer.Close()
	}

	shutdownTimeout := 5 * time.Second

	if cfg.RPC.Enable {
		if err := rpcServer.Close(shutdownTimeout); err != nil {
			Logger.Fatal("Failed to shutdown RPC server gracefully: ", err)
		}
	}

	if cfg.Metrics.Enable {
		if err := metricsServer.Close(shutdownTimeout); err != nil {
			Logger.Fatal("Failed to shutdown Metrics server gracefully: ", err)
		}
	}

	if cfg.REST.Enable {
		if err := restServer.Close(shutdownTimeout); err != nil {
			Logger.Fatal("Failed to shutdown REST server gracefully: ", err)
		}
	}
}

func checkErrChs(errChs []chan error) {
	for _, errCh := range errChs {
		for {
			if err := <-errCh; err != nil {
				Logger.Fatal(err)
			}
		}
	}
}
