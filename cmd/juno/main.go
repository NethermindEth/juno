package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"syscall"
	"time"

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
	"github.com/torquem-ch/mdbx-go/mdbx"
	"github.com/urfave/cli/v2"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

//go:embed long.txt
var longMsg string

var shutdownTimeout = 5 * time.Second

const (
	mdbxOptMaxDb uint64 = 100
	mdbxFlags    uint   = 0
)

var (
	mdbxEnv *mdbx.Env

	rpcServer     *rpc.Server
	metricsServer *metric.Server
	restServer    *rest.Server

	feederGatewayClient *feeder.Client

	stateSynchronizer *starknet.Synchronizer

	contractHashManager *contracthash.Manager
	abiManager          *abi.Manager
	stateManager        *state.Manager
	transactionManager  *transaction.Manager
	blockManager        *block.Manager
)

func main() {
	// Setup interrupt handling as soon as possible
	setupInterruptHandler()

	app := &cli.App{
		Name:   "juno",
		Usage:  "Decentralize StarkNet.",
		Flags:  flags,
		Action: cli.ActionFunc(juno),
		// Prepend our ascii art to the default template
		CustomAppHelpTemplate: longMsg + cli.AppHelpTemplate,
	}

	sort.Sort(cli.FlagsByName(app.Flags))

	if err := app.Run(os.Args); err != nil {
		if Logger == nil {
			// The logger has not been configured yet.
			// This can only happen very early.
			fmt.Println("closing...", err)
		} else {
			Logger.With("error", err).Error("closing...")
		}
	}
	shutdown()
}

func juno(ctx *cli.Context) error {
	fmt.Println(longMsg)

	config, err := setupConfig(ctx)
	if err != nil {
		fmt.Println("error initializing configuration:", err)
		os.Exit(1)
	}

	// All of these functions will quit the program if there are errors.
	setupLogger(config)
	setupDatabaseManagers(config)
	setupFeederGateway(config)
	setupServers(config)
	setupStateSynchronizer(config)

	errChs := []chan error{make(chan error), make(chan error), make(chan error)}
	rpcServer.ListenAndServe(errChs[0])
	metricsServer.ListenAndServe(errChs[1])
	restServer.ListenAndServe(errChs[2])

	if err := stateSynchronizer.UpdateState(); err != nil {
		Logger.Fatal("Failed to start State Synchronizer: ", err.Error())
	}

	checkErrChs(errChs)
	return nil
}

func setupLogger(config *JunoConfig) {
	loggerConfig := zap.NewProductionConfig()

	// Timestamp format (ISO8601) and time zone (UTC)
	loggerConfig.EncoderConfig.EncodeTime = zapcore.TimeEncoder(func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		// notest
		enc.AppendString(t.UTC().Format("2006-01-02T15:04:05Z0700"))
	})

	// Output encoding
	loggerConfig.Encoding = "console"
	if config.Log.EnableJson {
		loggerConfig.Encoding = "json"
	}

	// Log level
	logLevel, err := zapcore.ParseLevel(config.Log.Level)
	if err != nil {
		fmt.Printf("failed to parse log level %s: %x\n", config.Log.Level, err)
		os.Exit(1)
	}
	loggerConfig.Level.SetLevel(logLevel)

	logger, err := loggerConfig.Build()
	if err != nil {
		fmt.Printf("failed to build logger: %x\n", err)
		os.Exit(1)
	}

	Logger = logger.Sugar()
}

func setupStateSynchronizer(config *JunoConfig) {
	if config.Starknet.Enable {
		var ethereumClient *ethclient.Client
		if !config.Starknet.SyncFeeder {
			var err error
			ethereumClient, err = ethclient.Dial(config.Ethereum.Node)
			if err != nil {
				Logger.Fatal("Failed to connect to ethereum client: %w", err)
			}
		}
		// Synchronizer for Starknet State
		synchronizerDb, err := db.NewMDBXDatabase(mdbxEnv, "SYNCHRONIZER")
		if err != nil {
			Logger.Fatal("Failed to start SYNCHRONIZER database: %w", err)
		}
		stateSynchronizer = starknet.NewSynchronizer(synchronizerDb, ethereumClient, feederGatewayClient,
			contractHashManager, abiManager, stateManager, transactionManager, blockManager)
	}
}

func setupServers(config *JunoConfig) {
	if config.RPC.Enable {
		rpcServer = rpc.NewServer(":"+strconv.FormatUint(uint64(config.RPC.Port), 10), feederGatewayClient, abiManager,
			stateManager, transactionManager, blockManager)
	}

	if config.Metrics.Enable {
		metricsServer = metric.SetupMetric(":" + strconv.FormatUint(uint64(config.Metrics.Port), 10))
	}

	if config.REST.Enable {
		restServer = rest.NewServer(":"+strconv.Itoa(config.REST.Port),
			config.Starknet.FeederEndpoint, config.REST.Prefix)
	}
}

func setupFeederGateway(config *JunoConfig) {
	feederGatewayClient = feeder.NewClient(config.Starknet.FeederEndpoint, "/feeder_gateway", nil)
}

func setupDatabaseManagers(config *JunoConfig) {
	var err error
	var dbName string

	mdbxEnv, err = db.NewMDBXEnv(config.DatabaseDirectory, mdbxOptMaxDb, mdbxFlags)
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

func setupInterruptHandler() {
	// Handle signal interrupts and exits.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func(sig chan os.Signal) {
		<-sig
		Logger.Info("Shutting down Juno...")
		shutdown()
		os.Exit(0)
	}(sig)
}

func shutdown() {
	contractHashManager.Close()
	abiManager.Close()
	stateManager.Close()
	transactionManager.Close()
	blockManager.Close()
	stateSynchronizer.Close()

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
			if err := <-errCh; err != nil {
				Logger.Fatal(err)
			}
		}
	}
}
