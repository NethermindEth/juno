package cli

import (
	_ "embed"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/NethermindEth/juno/pkg/jsonrpc"

	"github.com/NethermindEth/juno/internal/cairovm"
	"github.com/NethermindEth/juno/internal/config"
	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/db/block"
	"github.com/NethermindEth/juno/internal/db/state"
	"github.com/NethermindEth/juno/internal/db/sync"
	"github.com/NethermindEth/juno/internal/db/transaction"
	. "github.com/NethermindEth/juno/internal/log"
	metric "github.com/NethermindEth/juno/internal/metrics/prometheus"
	"github.com/NethermindEth/juno/internal/rpc"
	"github.com/NethermindEth/juno/internal/rpc/starknet"
	syncService "github.com/NethermindEth/juno/internal/sync"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/NethermindEth/juno/pkg/rest"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/torquem-ch/mdbx-go/mdbx"
)

const (
	mdbxOptMaxDb uint64 = 100
	mdbxFlags    uint   = 0
	minPort      int    = 1_024
	maxPort      int    = 49_151
)

// Cobra configuration.
var (
	// cfgFile is the path of the juno configuration file.
	cfgFile string
	// dataDir is the path of the directory to read and save user-specific
	// application data
	dataDir string
	// longMsg is the long message shown in the "juno --help" output.
	//go:embed long.txt
	longMsg string
)

var (
	mdbxEnv *mdbx.Env

	rpcServer     *rpc.HttpRpc
	metricsServer *metric.Server
	restServer    *rest.Server

	feederGatewayClient *feeder.Client

	synchronizer   *syncService.Synchronizer
	virtualMachine *cairovm.VirtualMachine

	stateManager       *state.Manager
	transactionManager *transaction.Manager
	blockManager       *block.Manager
	syncManager        *sync.Manager
)

const shutdownTimeout = 5 * time.Second

// rootCmd is the root command of the application.
var rootCmd = &cobra.Command{
	Use:   "juno [options]",
	Short: "Starknet client implementation in Go.",
	Run:   juno,
}

// Execute handle flags for Cobra execution.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		Logger.With("Error", err).Error("Failed to execute CLI.")
	}
}

// init defines flags and handles configuration.
func init() {
	fmt.Println(longMsg)
	// Set the functions to be run when rootCmd.Execute() is called.
	initConfig()

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", fmt.Sprintf(
		"config file (default is %s)", filepath.Join(config.Dir, "juno.yaml")))
	rootCmd.PersistentFlags().StringVar(&dataDir, "dataDir", "", fmt.Sprintf(
		"data path (default is %s)", config.DataDir))
}

func juno(_ *cobra.Command, _ []string) {
	Logger.With(
		"Database Path", config.Runtime.DbPath,
		"Rpc Port", config.Runtime.RPC.Port,
		"Rpc Enabled", config.Runtime.RPC.Enabled,
		"Rest Port", config.Runtime.REST.Port,
		"Rest Enabled", config.Runtime.REST.Enabled,
		"Rest Prefix", config.Runtime.REST.Prefix,
	).Info("Juno config values:")

	setupInterruptHandler()
	setupDatabaseManagers()
	setupFeederGateway()
	setupSynchronizer()
	setupVirtualMachine()
	setupServers()

	const numOfErrCh = 3
	errChs := make([]chan error, numOfErrCh)
	for i := 0; i < numOfErrCh; i++ {
		errChs[i] = make(chan error)
	}

	if config.Runtime.RPC.Enabled {
		rpcServer.ListenAndServe(errChs[0])
	}
	if config.Runtime.Metrics.Enabled {
		metricsServer.ListenAndServe(errChs[1])
	}
	if config.Runtime.REST.Enabled {
		restServer.ListenAndServe(errChs[2])
	}
	if config.Runtime.Starknet.Enabled {
		synchronizer.Run()
	}

	checkErrChs(errChs)
}

func setupVirtualMachine() {
	virtualMachine = cairovm.New(stateManager)
}

func setupSynchronizer() {
	synchronizer = syncService.NewSynchronizer(feederGatewayClient, syncManager, stateManager, blockManager,
		transactionManager)
}

func setupServers() {
	var err error
	if config.Runtime.RPC.Enabled {
		checkPort("JSON-RPC", config.Runtime.RPC.Port)
		starknetApi := starknet.New(stateManager, blockManager, transactionManager, synchronizer, virtualMachine)
		jsonRpc := jsonrpc.NewJsonRpc()
		handlers := []struct {
			name       string
			function   any
			paramNames []string
		}{
			{"starknet_getBlockWithTxHashes", starknetApi.GetBlockWithTxHashes, []string{"block_id"}},
			{"starknet_getBlockWithTxs", starknetApi.GetBlockWithTxs, []string{"block_id"}},
			{"starknet_getStateUpdate", starknetApi.GetStateUpdate, []string{"block_id"}},
			{"starknet_getStorageAt", starknetApi.GetStorageAt, []string{"block_id", "address", "key"}},
			{"starknet_getTransactionByHash", starknetApi.GetTransactionByHash, []string{"transaction_hash"}},
			{"starknet_getTransactionByBlockIdAndIndex", starknetApi.GetTransactionByBlockIdAndIndex, []string{"block_id", "index"}},
			{"starknet_getTransactionReceipt", starknetApi.GetTransactionReceipt, []string{"transaction_hash"}},
			{"starknet_getClass", starknetApi.GetClass, []string{"class_hash"}},
			{"starknet_getClassHashAt", starknetApi.GetClassHashAt, []string{"block_id", "address"}},
			{"starknet_getBlockTransactionCount", starknetApi.GetBlockTransactionCount, []string{"block_id"}},
			{"starknet_call", starknetApi.Call, []string{"block_id", "request"}},
			{"starknet_estimateFee", starknetApi.EstimateFee, []string{"block_id", "request"}},
			{"starknet_blockNumber", starknetApi.BlockNumber, nil},
			{"starknet_blockHashAndNumber", starknetApi.BlockHashAndNumber, nil},
			{"starknet_chainId", starknetApi.ChainId, nil},
			{"starkent_pendingTrnasactions", starknetApi.PendingTransactions, nil},
			{"starknet_protocolVersion", starknetApi.ProtocolVersion, nil},
			{"starknet_syncing", starknetApi.Syncing, nil},
		}
		for _, handler := range handlers {
			if err := jsonRpc.RegisterFunc(handler.name, handler.function, handler.paramNames...); err != nil {
				Logger.With("Error", err).Error("Failed to register RPC handler.")
			}
		}
		rpcServer, err = rpc.NewHttpRpc(":"+strconv.Itoa(config.Runtime.RPC.Port), "/rpc", jsonRpc)
		if err != nil {
			Logger.Fatal("Failed to initialise RPC Server", err)
		}
	}

	if config.Runtime.Metrics.Enabled {
		checkPort("Metrics", config.Runtime.Metrics.Port)
		metricsServer = metric.SetupMetric(":" + strconv.Itoa(config.Runtime.Metrics.Port))
	}

	if config.Runtime.REST.Enabled {
		checkPort("API", config.Runtime.REST.Port)
		restServer = rest.NewServer(":"+strconv.Itoa(config.Runtime.REST.Port),
			config.Runtime.Starknet.FeederGateway, config.Runtime.REST.Prefix)
	}
}

func checkPort(server string, port int) {
	if port < minPort || port > maxPort {
		Logger.Fatalf("%s port must be between %d and %d", server, minPort, maxPort)
	}
}

func setupFeederGateway() {
	feederGatewayClient = feeder.NewClient(config.Runtime.Starknet.FeederGateway, "/feeder_gateway", nil)
}

func setupDatabaseManagers() {
	var err error
	var dbName string

	mdbxEnv, err = db.NewMDBXEnv(config.Runtime.DbPath, mdbxOptMaxDb, mdbxFlags)
	if err != nil {
		Logger.Fatal("Failed to create MDBX Database environment: ", err)
	}

	logDBErr := func(name string, err error) {
		if err != nil {
			Logger.Fatalf("Failed to create %s database: %s", name, err.Error())
		}
	}

	dbName = "SYNC"
	syncDb, err := db.NewMDBXDatabase(mdbxEnv, dbName)
	logDBErr(dbName, err)

	dbName = "CONTRACT_DEF"
	contractDefDb, err := db.NewMDBXDatabase(mdbxEnv, dbName)
	logDBErr(dbName, err)

	dbName = "STATE"
	stateDb, err := db.NewMDBXDatabase(mdbxEnv, dbName)
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

	syncManager = sync.NewManager(syncDb)
	stateManager = state.NewManager(stateDb, contractDefDb)
	transactionManager = transaction.NewManager(txDb, receiptDb)
	blockManager = block.NewManager(blockDb)
}

func setupInterruptHandler() {
	// Handle signal interrupts and exits.
	sig := make(chan os.Signal)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func(sig chan os.Signal) {
		<-sig
		Logger.Info("Shutting down...")
		shutdown()
		os.Exit(0)
	}(sig)
}

func shutdown() {
	stateManager.Close()
	transactionManager.Close()
	blockManager.Close()
	stateManager.Close()
	syncManager.Close()
	synchronizer.Close()

	if err := rpcServer.Close(shutdownTimeout); err != nil {
		Logger.Fatal("Failed to shutdown RPC server gracefully: ", err.Error())
	}

	if err := metricsServer.Close(shutdownTimeout); err != nil {
		Logger.Fatal("Failed to shutdown Metrics server gracefully: ", err.Error())
	}

	if err := restServer.Close(shutdownTimeout); err != nil {
		Logger.Fatal("Failed to shutdown REST server gracefully: ", err.Error())
	}
}

func checkErrChs(errChs []chan error) {
	for _, errCh := range errChs {
		for {
			if err, ok := <-errCh; err != nil {
				Logger.Fatal(err)
			} else if !ok {
				continue
			}
		}
	}
}

// initConfig reads in Config file or environment variables if set.
func initConfig() {
	if dataDir != "" {
		info, err := os.Stat(dataDir)
		if err != nil || !info.IsDir() {
			dataDir = config.DataDir
			Logger.Infof("Invalid data directory. The default data directory (%s) will be used.", dataDir)
		}
	}
	if cfgFile != "" {
		// Use Config file specified by the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Use the default path for user configuration.
		viper.AddConfigPath(config.Dir)
		viper.SetConfigType("yaml")
		viper.SetConfigName("juno")
	}

	// Check whether the environment variables match any of the existing
	// keys and loads them if they are found.
	viper.AutomaticEnv()

	err := viper.ReadInConfig()
	if err == nil {
		Logger.Infof("Using config file: %s.", viper.ConfigFileUsed())
	} else {
		Logger.Info("Config file not found.")
		if !config.Exists() {
			config.New()
		}
		viper.SetConfigFile(filepath.Join(config.Dir, "juno.yaml"))
		err = viper.ReadInConfig()
		if err != nil {
			Logger.Fatal("Failed to read in config file after generation.")
		}
	}

	// Unmarshal and log runtime config instance.
	err = viper.Unmarshal(&config.Runtime)
	if err != nil {
		Logger.Fatal("Failed to parse runtime configuration.")
	}

	// Configure logger - we want the logger to be created right after the config has been set
	enableJsonOutput := config.Runtime.Logger.EnableJsonOutput
	verbosityLevel := config.Runtime.Logger.VerbosityLevel
	err = ReplaceGlobalLogger(enableJsonOutput, verbosityLevel)
	if err != nil {
		Logger.Fatal("Failed to initialise global logger.")
	}

	Logger.With(
		"Verbosity Level", verbosityLevel,
		"Json Output", enableJsonOutput,
	).Info("Logger settings:")
}
