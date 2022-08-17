package juno

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"

	"github.com/NethermindEth/juno/internal/config"
	"github.com/NethermindEth/juno/internal/juno"

	"github.com/NethermindEth/juno/internal/cairovm"
	"github.com/NethermindEth/juno/internal/db/block"
	"github.com/NethermindEth/juno/internal/db/state"
	"github.com/NethermindEth/juno/internal/db/sync"
	"github.com/NethermindEth/juno/internal/db/transaction"
	metric "github.com/NethermindEth/juno/internal/metrics/prometheus"
	"github.com/NethermindEth/juno/internal/rpc"
	syncService "github.com/NethermindEth/juno/internal/sync"
	"github.com/spf13/cobra"
	"github.com/torquem-ch/mdbx-go/mdbx"
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

var JunoNode juno.StarkNetNode

var (
	mdbxEnv *mdbx.Env

	rpcServer     *rpc.HttpRpc
	metricsServer *metric.Server

	synchronizer   *syncService.Synchronizer
	virtualMachine *cairovm.VirtualMachine

	stateManager       *state.Manager
	transactionManager *transaction.Manager
	blockManager       *block.Manager
	syncManager        *sync.Manager
)

func NewCmd(newStarkNetNodeFn juno.NewStarkNetNodeFn) *cobra.Command {
	var cfgFile string

	configF := "config"
	verbosityF := "verbosity"
	rpcPortF := "rpc-port"
	metricsPortF := "metrics-port"
	dbPathF := "db-path"
	networkF := "network"
	ethNodeF := "eth-node"

	defaultConfig := ""
	defaultVerbosity := "info"
	defaultRpcPort := uint16(6060)
	defaultMetricsPort := uint16(0)
	defaultDbPath := ""
	defaultNetwork := config.GOERLI
	defaultEthNode := ""

	configFlagUsage := "The yaml configuration file."
	verbosityFlagUsage := "Verbosity of the logs. Options: debug, info, warn, error, dpanic, " +
		"panic, fatal."
	rpcPortUsage := "The port on which the RPC server will listen for requests. " +
		"Warning: this exposes the node to external requests and potentially DoS attacks."
	metricsPortUsage := "Enables the metrics server and listens on the provided port."
	dbPathUsage := "Location of the database files."
	networkUsage := fmt.Sprintf("Available StarkNet networks. Options: %d = %v and %d = %v",
		config.GOERLI, config.GOERLI, config.MAINNET, config.MAINNET)
	ethNodeUsage := "The Ethereum endpoint to synchronise with. " +
		"If unset feeder gateway will be used."

	junoCmd := &cobra.Command{
		Use:   "juno [flags]",
		Short: "StarkNet client implementation in Go.",
	}

	junoCmd.Flags().StringVar(&cfgFile, configF, defaultConfig, configFlagUsage)
	junoCmd.Flags().String(verbosityF, defaultVerbosity, verbosityFlagUsage)
	junoCmd.Flags().Uint16(rpcPortF, defaultRpcPort, rpcPortUsage)
	junoCmd.Flags().Uint16(metricsPortF, defaultMetricsPort, metricsPortUsage)
	junoCmd.Flags().String(dbPathF, defaultDbPath, dbPathUsage)
	junoCmd.Flags().Uint8(networkF, uint8(defaultNetwork), networkUsage)
	junoCmd.Flags().String(ethNodeF, defaultEthNode, ethNodeUsage)

	junoCmd.RunE = func(cmd *cobra.Command, args []string) error {
		v := viper.New()

		if cfgFile != "" {
			v.SetConfigName(cfgFile)
			if err := v.ReadInConfig(); err != nil {
				return err
			}
		}

		if err := v.BindPFlags(cmd.Flags()); err != nil {
			return nil
		}

		if _, err := fmt.Fprintf(cmd.OutOrStdout(), greeting); err != nil {
			return err
		}

		// If the db-path is not set by the user via configuration file or flag
		// set the db-path according to the network.
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		junoDir := ".juno"

		if v.GetString(dbPathF) == "" {
			network := config.Network(v.GetUint(networkF))
			v.Set(dbPathF, filepath.Join(homeDir, junoDir, network.String()))
		}

		junoCfg := new(config.Juno)
		if err = v.Unmarshal(junoCfg); err != nil {
			return err
		}

		JunoNode, err = newStarkNetNodeFn(junoCfg)
		if err != nil {
			return err
		}
		return nil
	}

	return junoCmd
}

func runJuno(cmd *cobra.Command, _ []string) error {
	// Initial config usgin

	//cfg := &config.Juno{}
	//// Start juno
	//juno(cfg)
	return nil
}

//type keyToEnvReplacer struct{}
//
//// Replace is used to conveniently convert from a viper key to an
//// environment variable. E.g. "database-path" --> "JUNO_DATABASE_PATH"
////
//// It is public to fulfill the viper.EnvKeyReplacer interface.
//// Not meant for external use.
//func (r *keyToEnvReplacer) Replace(key string) string {
//	return "JUNO_" + strings.ToUpper(strings.ReplaceAll(key, "-", "_"))
//}
//
//// Loads the viper configuration and handles precedence in the following order:
////
//// 1. CLI params
//// 2. Environment variables
//// 3. Config file
//// 4. Defaults
////
//// Inspired by https://github.com/carolynvs/stingoftheviper/blob/e0d04fd2334bdf677a7f8825404a70e3c2c7e7d0/main.go
//func loadConfig(cmd *cobra.Command, configFile *string) error {
//	v := viper.NewWithOptions(viper.KeyDelimiter("-"), viper.EnvKeyReplacer(&keyToEnvReplacer{}))
//
//	// The `--config` flag is unique. We need to evaluate its
//	// argument/env var before loading the configuration file.
//	setFlagValue(cmd, v, cmd.Flag("config"))
//
//	v.AddConfigPath(filepath.Dir(*configFile))
//	v.SetConfigType("yaml")
//	v.SetConfigName(filepath.Base(*configFile))
//
//	if err := v.ReadInConfig(); err != nil {
//		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
//			return err
//		}
//		fmt.Printf("config file not found at %s: falling back to CLI params, environment vars, and defaults\n", *configFile)
//	}
//
//	// Copy environment vars and config file values to the
//	// config struct.
//	cmd.Flags().VisitAll(func(f *pflag.Flag) { setFlagValue(cmd, v, f) })
//
//	return nil
//}
//
//// setFlagValue binds each cobra flag to its associated viper
//// configuration (config file and environment variable)
//func setFlagValue(cmd *cobra.Command, v *viper.Viper, f *pflag.Flag) {
//	// Bind flag to environment variable
//	v.BindEnv(f.Name)
//
//	// Apply the viper config value to the flag when the flag is not
//	// set and viper has a value
//	if !f.Changed && v.IsSet(f.Name) {
//		val := v.Get(f.Name)
//		cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val))
//	}
//}
//
//// juno is the main entrypoint for the Juno node.
//func juno(cfg *config.Juno) {
//	fmt.Printf("using config: %+v\n\n", cfg)
//
//	// Configure the logger first so we can use it
//	setupLogger(&cfg.Log)
//	setupInterruptHandler(cfg)
//	setupDatabase(&cfg.Database)
//	feederClient := setupFeederGateway(&cfg.Sync)
//	virtualMachine = cairovm.New(stateManager)
//
//	errChs := make([]chan error, 0)
//
//	if cfg.Sync.Enable {
//		errChs = append(errChs, make(chan error))
//		setupSynchronizer(&cfg.Sync, feederClient, errChs[len(errChs)-1])
//	} else {
//		// Currently, Juno is only useful for storing StarkNet
//		// state locally. We notify the user of this. We don't
//		// exit since some RPCs are useful on a stale database.
//		fmt.Println("StarkNet synchronization is disabled. To enable it, use the --sync-enable flag.")
//	}
//	if cfg.Rpc.Enable {
//		errChs = append(errChs, make(chan error))
//		setupRpc(&cfg.Rpc, synchronizer, errChs[len(errChs)-1])
//	}
//	if cfg.Metrics.Enable {
//		errChs = append(errChs, make(chan error))
//		setupMetrics(&cfg.Metrics, errChs[len(errChs)-1])
//	}
//
//	// Wait until error
//	checkErrChs(errChs)
//}
//
//func setupVirtualMachine() {
//	virtualMachine = cairovm.New(stateManager)
//}
//
//func setupLogger(cfg *config.Log) {
//	if err := ReplaceGlobalLogger(cfg.Json, cfg.Level, !cfg.NoColor); err != nil {
//		fmt.Printf("failed to initialize logger: %s\n", err)
//		os.Exit(1)
//	}
//}
//
//func setupSynchronizer(cfg *config.Sync, feederClient *feeder.Client, errChan chan error) {
//	synchronizer = syncService.NewSynchronizer(cfg, feederClient, syncManager, stateManager, blockManager,
//		transactionManager)
//	synchronizer.Run(cfg.Trusted, errChan)
//}
//
//func setupRpc(cfg *config.Rpc, synchronizer *syncService.Synchronizer, errChan chan error) {
//	checkPort("JSON-RPC", cfg.Port)
//	starknetApi := starknet.New(stateManager, blockManager, transactionManager, synchronizer, virtualMachine)
//	jsonRpc := jsonrpc.NewJsonRpc()
//	handlers := []struct {
//		name       string
//		function   any
//		paramNames []string
//	}{
//		{"starknet_getBlockWithTxHashes", starknetApi.GetBlockWithTxHashes, []string{"block_id"}},
//		{"starknet_getBlockWithTxs", starknetApi.GetBlockWithTxs, []string{"block_id"}},
//		{"starknet_getStateUpdate", starknetApi.GetStateUpdate, []string{"block_id"}},
//		{"starknet_getStorageAt", starknetApi.GetStorageAt, []string{"block_id", "contract_address", "key"}},
//		{"starknet_getTransactionByHash", starknetApi.GetTransactionByHash, []string{"transaction_hash"}},
//		{"starknet_getTransactionByBlockIdAndIndex", starknetApi.GetTransactionByBlockIdAndIndex, []string{"block_id", "index"}},
//		{"starknet_getTransactionReceipt", starknetApi.GetTransactionReceipt, []string{"transaction_hash"}},
//		{"starknet_getClass", starknetApi.GetClass, []string{"class_hash"}},
//		{"starknet_getClassHashAt", starknetApi.GetClassHashAt, []string{"block_id", "address"}},
//		{"starknet_getBlockTransactionCount", starknetApi.GetBlockTransactionCount, []string{"block_id"}},
//		{"starknet_call", starknetApi.Call, []string{"block_id", "request"}},
//		{"starknet_estimateFee", starknetApi.EstimateFee, []string{"block_id", "request"}},
//		{"starknet_blockNumber", starknetApi.BlockNumber, nil},
//		{"starknet_blockHashAndNumber", starknetApi.BlockHashAndNumber, nil},
//		{"starknet_chainId", starknetApi.ChainId, nil},
//		{"starknet_pendingTransactions", starknetApi.PendingTransactions, nil},
//		{"starknet_protocolVersion", starknetApi.ProtocolVersion, nil},
//		{"starknet_syncing", starknetApi.Syncing, nil},
//	}
//	for _, handler := range handlers {
//		if err := jsonRpc.RegisterFunc(handler.name, handler.function, handler.paramNames...); err != nil {
//			Logger.With("Error", err).Error("Failed to register RPC handler.")
//		}
//	}
//	server, err := rpc.NewHttpRpc(":"+strconv.FormatUint(uint64(cfg.Port), 10), "/rpc", jsonRpc)
//	if err != nil {
//		Logger.Fatal("Failed to initialise RPC Server", err)
//	}
//	rpcServer = server
//	rpcServer.ListenAndServe(errChan)
//}
//
//func setupMetrics(cfg *config.Metrics, errChan chan error) {
//	checkPort("Metrics", cfg.Port)
//	metricsServer = metric.SetupMetric(":" + strconv.FormatUint(uint64(cfg.Port), 10))
//	metricsServer.ListenAndServe(errChan)
//}
//
//func checkPort(server string, port uint) {
//	minPort, maxPort := uint(1_024), uint(49_151)
//	if port < minPort || port > maxPort {
//		Logger.Fatalf("%s port must be between %d and %d", server, minPort, maxPort)
//	}
//}
//
//func setupFeederGateway(cfg *config.Sync) *feeder.Client {
//	return feeder.NewClient(cfg.Sequencer, "/feeder_gateway", nil)
//}
//
//func setupDatabase(cfg *config.Database) {
//	var err error
//	var dbName string
//
//	mdbxEnv, err = db.NewMDBXEnv(cfg.Path, 100, 0)
//	if err != nil {
//		Logger.Fatal("Failed to create MDBX Database environment: ", err)
//	}
//
//	logDBErr := func(name string, err error) {
//		if err != nil {
//			Logger.Fatalf("Failed to create %s database: %s", name, err)
//		}
//	}
//
//	dbName = "SYNC"
//	syncDb, err := db.NewMDBXDatabase(mdbxEnv, dbName)
//	logDBErr(dbName, err)
//
//	dbName = "CONTRACT_DEF"
//	contractDefDb, err := db.NewMDBXDatabase(mdbxEnv, dbName)
//	logDBErr(dbName, err)
//
//	dbName = "STATE"
//	stateDb, err := db.NewMDBXDatabase(mdbxEnv, dbName)
//	logDBErr(dbName, err)
//
//	dbName = "TRANSACTION"
//	txDb, err := db.NewMDBXDatabase(mdbxEnv, dbName)
//	logDBErr(dbName, err)
//
//	dbName = "RECEIPT"
//	receiptDb, err := db.NewMDBXDatabase(mdbxEnv, dbName)
//	logDBErr(dbName, err)
//
//	dbName = "BLOCK"
//	blockDb, err := db.NewMDBXDatabase(mdbxEnv, dbName)
//	logDBErr(dbName, err)
//
//	syncManager = sync.NewManager(syncDb)
//	stateManager = state.NewManager(stateDb, contractDefDb)
//	transactionManager = transaction.NewManager(txDb, receiptDb)
//	blockManager = block.NewManager(blockDb)
//}
//
//func setupInterruptHandler(cfg *config.Juno) {
//	// Handle signal interrupts and exits.
//	sig := make(chan os.Signal, 1)
//	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
//	go func(sig chan os.Signal) {
//		<-sig
//		Logger.Info("Shutting down Juno...")
//		shutdown(cfg)
//		os.Exit(0)
//	}(sig)
//}
//
//func shutdown(cfg *config.Juno) {
//	stateManager.Close()
//	transactionManager.Close()
//	blockManager.Close()
//	stateManager.Close()
//	syncManager.Close()
//	if cfg.Sync.Enable {
//		synchronizer.Close()
//	}
//
//	shutdownTimeout := 5 * time.Second
//
//	if cfg.Rpc.Enable {
//		if err := rpcServer.Close(shutdownTimeout); err != nil {
//			Logger.Fatal("Failed to shutdown RPC server gracefully: ", err)
//		}
//	}
//
//	if cfg.Metrics.Enable {
//		if err := metricsServer.Close(shutdownTimeout); err != nil {
//			Logger.Fatal("Failed to shutdown Metrics server gracefully: ", err)
//		}
//	}
//}
//
//func checkErrChs(errChs []chan error) {
//	for _, errCh := range errChs {
//		for {
//			if err := <-errCh; err != nil {
//				Logger.Fatal(err)
//			}
//		}
//	}
//}
