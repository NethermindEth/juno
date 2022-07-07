package cli

// notest
import (
	_ "embed"
	"fmt"
	"github.com/NethermindEth/juno/internal/config"
	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/db/abi"
	"github.com/NethermindEth/juno/internal/db/block"
	"github.com/NethermindEth/juno/internal/db/state"
	"github.com/NethermindEth/juno/internal/db/transaction"
	"github.com/NethermindEth/juno/internal/errpkg"
	"github.com/NethermindEth/juno/internal/log"
	metric "github.com/NethermindEth/juno/internal/metrics/prometheus"
	"github.com/NethermindEth/juno/internal/process"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/NethermindEth/juno/pkg/rest"
	"github.com/NethermindEth/juno/pkg/rpc"
	"github.com/NethermindEth/juno/pkg/starknet"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

const (
	contextTimeoutDuration = 5 * time.Second
)

// Cobra configuration.
var (
	// cfgFile is the path of the juno configuration file.
	cfgFile string
	// dataDir is the path of the directory to read and save user-specific application data
	dataDir string
	// longMsg is the long message shown in the "juno --help" output.
	//go:embed long.txt
	longMsg        string
	processHandler *process.Handler
)

// rootCmd is the root command of the application.
var rootCmd = &cobra.Command{
	Use:   "juno [options]",
	Short: "Starknet client implementation in Go.",
	Run:   juno,
}

var (
	rpcServer           *rpc.Server
	metricsServer       *metric.Server
	restServer          *rest.Server
	feederGatewayClient *feeder.Client
	stateSynchronizer   *starknet.Synchronizer

	abiManager         *abi.Manager
	stateManager       *state.Manager
	transactionManager *transaction.Manager
	blockManager       *block.Manager
)

// Execute handle flags for Cobra execution.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Default.With("Error", err).Error("Failed to execute CLI.")
	}
}

// init defines flags and handles configuration.
func init() {
	fmt.Println(longMsg)
	// Set the functions to be run when rootCmd.Execute() is called.
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", fmt.Sprintf(
		"config file (default is %s)", filepath.Join(config.ConfigurationDir, "juno.yaml")))
	rootCmd.PersistentFlags().StringVar(&dataDir, "dataDir", "", fmt.Sprintf(
		"data path (default is %s)", config.DataDir))
}

func juno(_ *cobra.Command, _ []string) {
	setupSignalInterruptHandler()

	// Setup client and managers
	feederGatewayClient = feeder.NewClient(config.Runtime.Starknet.FeederGateway, utils.FeederGatewayApiPrefix, nil)

	abiDB, err := db.NewMDBXDatabase("ABI")
	if err != nil {
		log.Default.With("Error", err).Fatal("Error creating the ABI database")
	}
	abiManager = abi.NewABIManager(abiDB)

	codeDB, err := db.NewMDBXDatabase("CODE")
	if err != nil {
		log.Default.With("Error", err).Fatal("Error creating the CODE database")
	}
	storageDB, err := db.NewMDBXDatabase("STORAGE")
	if err != nil {
		log.Default.With("Error", err).Fatal("Error creating the STORAGE database")
	}
	stateManager = state.NewStateManager(codeDB, db.NewBlockSpecificDatabase(storageDB))

	txDB, err := db.NewMDBXDatabase("TRANSACTION")
	if err != nil {
		log.Default.With("Error", err).Fatal("Error creating the TRANSACTION database")
	}
	receiptDB, err := db.NewMDBXDatabase("RECEIPT")
	if err != nil {
		log.Default.With("Error", err).Fatal("Error creating the RECEIPT database")
	}
	transactionManager = transaction.NewManager(txDB, receiptDB)

	blockDB, err := db.NewMDBXDatabase("BLOCK")
	if err != nil {
		log.Default.With("Error", err).Fatal("Error creating the BLOCK database")
	}
	blockManager = block.NewManager(blockDB)

	// Initialise servers and state synchronisation
	if config.Runtime.RPC.Enabled {
		rpcServer = rpc.NewServer(":"+strconv.Itoa(config.Runtime.RPC.Port), feederGatewayClient, abiManager,
			stateManager, transactionManager, blockManager)
	}

	if config.Runtime.Metrics.Enabled {
		metricsServer = metric.SetupMetric(":" + strconv.Itoa(config.Runtime.Metrics.Port))
	}

	if config.Runtime.Starknet.Enabled {
		var ethereumClient *ethclient.Client

		if !config.Runtime.Starknet.ApiSync {
			var err error
			ethereumClient, err = ethclient.Dial(config.Runtime.Ethereum.Node)
			if err != nil {
				log.Default.With("Error", err).Fatal("Unable to connect to Ethereum Client")
			}
		}

		synchronizerDb, err := db.NewMDBXDatabase("SYNCHRONIZER")
		if err != nil {
			log.Default.With("Error", err).Fatal("Error starting the SYNCHRONIZER database")
		}
		stateSynchronizer = starknet.NewSynchronizer(synchronizerDb, ethereumClient, feederGatewayClient, abiManager,
			stateManager, transactionManager, blockManager)
	}

	if config.Runtime.REST.Enabled {
		restServer = rest.NewServer(":"+strconv.Itoa(config.Runtime.REST.Port),
			config.Runtime.Starknet.FeederGateway, config.Runtime.REST.Prefix)
	}

	// Start servers and state synchronisation
	if err := rpcServer.ListenAndServe(); err != nil {
		log.Default.With("Error", err).Fatal("Error while calling rpcServer.ListenAndServe()")
	}

	if err := metricsServer.ListenAndServe(); err != nil {
		log.Default.With("Error", err).Fatal("Error while calling metrics.ListenAndServe()")
	}

	if err := restServer.ListenAndServe(); err != nil {
		log.Default.With("Error", err).Fatal("Error while calling rest.ListenAndServe()")
	}

	if err := stateSynchronizer.UpdateState(); err != nil {
		log.Default.With("Error", err).Fatal("Error while calling stateSynchronizer.UpdateState()")
	}

	processHandler = process.NewHandler()
	//
	//processHandler.Add("Contract Hash Storage Service", false, services.ContractHashService.Run,
	//	services.ContractHashService.Close)
}

func stop() {
	if err := rpcServer.Close(contextTimeoutDuration); err != nil {
		log.Default.With("Error", err).Fatal("Error while calling rpcServer.Close()")
	}

	if err := metricsServer.Close(contextTimeoutDuration); err != nil {
		log.Default.With("Error", err).Fatal("Error while calling metricsServer.Close()")
	}

	if err := restServer.Close(contextTimeoutDuration); err != nil {
		log.Default.With("Error", err).Fatal("Error while calling restServer.Close()")
	}

	if err := restServer.Close(contextTimeoutDuration); err != nil {
		log.Default.With("Error", err).Fatal("Error while calling restServer.Close()")
	}

	stateSynchronizer.Close()
	abiManager.Close()
	stateManager.Close()
	transactionManager.Close()
	blockManager.Close()
}

// Todo: ensure shutdown happens gracefully
func setupSignalInterruptHandler() {
	sig := make(chan os.Signal)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	go func(sig chan os.Signal) {
		<-sig
		log.Default.Info("Trying to close...")
		stop()
		os.Exit(0)
	}(sig)
}

func cleanup() {
	processHandler.Close()
	log.Default.Info("App closing...Bye!!!")
}

// initConfig reads in Config file or environment variables if set.
func initConfig() {
	if dataDir != "" {
		info, err := os.Stat(dataDir)
		if err != nil || !info.IsDir() {
			log.Default.Info("Invalid data directory. The default data directory will be used")
			dataDir = config.DataDir
		}
	}
	if cfgFile != "" {
		// Use Config file specified by the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Use the default path for user configuration.
		viper.AddConfigPath(config.ConfigurationDir)
		viper.SetConfigType("yaml")
		viper.SetConfigName("juno")
	}

	// Check whether the environment variables match any of the existing
	// keys and loads them if they are found.
	viper.AutomaticEnv()

	err := viper.ReadInConfig()
	if err == nil {
		log.Default.With("File", viper.ConfigFileUsed()).Info("Using config file:")
	} else {
		log.Default.Info("Config file not found.")
		if !config.Exists() {
			config.New()
		}
		viper.SetConfigFile(filepath.Join(config.ConfigurationDir, "juno.yaml"))
		err = viper.ReadInConfig()
		errpkg.CheckFatal(err, "Failed to read in Config after generation.")
	}

	// Unmarshal and log runtime config instance.
	err = viper.Unmarshal(&config.Runtime)
	errpkg.CheckFatal(err, "Unable to unmarshal runtime config instance.")
	log.Default.With(
		"Database Path", config.Runtime.DbPath,
		"Rpc Port", config.Runtime.RPC.Port,
		"Rpc Enabled", config.Runtime.RPC.Enabled,
		"Rest Port", config.Runtime.REST.Port,
		"Rest Enabled", config.Runtime.REST.Enabled,
		"Rest Prefix", config.Runtime.REST.Prefix,
	).Info("Config values.")
}
