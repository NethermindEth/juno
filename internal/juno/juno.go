package juno

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/NethermindEth/juno/internal/cairovm"
	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/db/block"
	"github.com/NethermindEth/juno/internal/db/state"
	"github.com/NethermindEth/juno/internal/db/sync"
	"github.com/NethermindEth/juno/internal/db/transaction"
	"github.com/NethermindEth/juno/internal/metrics/prometheus"
	"github.com/NethermindEth/juno/internal/rpc"
	"github.com/NethermindEth/juno/internal/rpc/starknet"
	syncer "github.com/NethermindEth/juno/internal/sync"
	"github.com/NethermindEth/juno/internal/utils"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/NethermindEth/juno/pkg/jsonrpc"
	"github.com/NethermindEth/juno/pkg/log"
)

// notest
type StarkNetNode interface {
	Run() error
	Shutdown() error
}

type NewStarkNetNodeFn func(cfg *Config) (StarkNetNode, error)

const (
	feederGatewaySuffix = "/feeder_gateway"
	rpcSuffix           = "/rpc"

	defaultMetricsPort = ":9090"

	shutdownTimeout = 5 * time.Second
)

var ErrUnknownNetwork = errors.New("unknown network")

// Config is the top-level juno configuration.
type Config struct {
	Verbosity    string        `mapstructure:"verbosity"`
	RpcPort      uint16        `mapstructure:"rpc-port"`
	Metrics      bool          `mapstructure:"metrics"`
	DatabasePath string        `mapstructure:"db-path"`
	Network      utils.Network `mapstructure:"network"`
	EthNode      string        `mapstructure:"eth-node"`
}

type Node struct {
	cfg *Config

	logger log.Logger

	feederClient   *feeder.Client
	virtualMachine *cairovm.VirtualMachine
	synchronizer   *syncer.Synchronizer

	syncManager        *sync.Manager
	stateManager       *state.Manager
	transactionManager *transaction.Manager
	blockManager       *block.Manager

	rpcServer     *rpc.HttpRpc
	metricsServer *prometheus.Server
}

func New(cfg *Config) (StarkNetNode, error) {
	if cfg.Network != utils.GOERLI && cfg.Network != utils.MAINNET {
		return nil, ErrUnknownNetwork
	}
	if cfg.DatabasePath == "" {
		dirPrefix, err := utils.DefaultDataDir()
		if err != nil {
			return nil, err
		}
		cfg.DatabasePath = filepath.Join(dirPrefix, cfg.Network.String())
	}

	return &Node{cfg: cfg}, nil
}

func (n *Node) Run() error {
	fmt.Printf("Running Juno with config: %s\n", fmt.Sprintf("%+v", *n.cfg))

	// Set up logging
	logger, err := log.NewProductionLogger(n.cfg.Verbosity)
	if err != nil {
		return err
	}
	n.logger = logger

	if err := utils.CreateDir(n.cfg.DatabasePath); err != nil {
		return err
	}

	// Set up database managers
	const optMaxDB, flags = uint64(100), uint(0)
	mdbxEnv, err := db.NewMDBXEnv(n.cfg.DatabasePath, optMaxDB, flags)
	if err != nil {
		return err
	}

	syncDb, err := db.NewMDBXDatabase(mdbxEnv, "SYNC")
	if err != nil {
		return err
	}
	n.syncManager = sync.NewManager(syncDb)

	contractDefDb, err := db.NewMDBXDatabase(mdbxEnv, "CONTRACT_DEF")
	if err != nil {
		return err
	}
	stateDb, err := db.NewMDBXDatabase(mdbxEnv, "STATE")
	if err != nil {
		return err
	}
	n.stateManager = state.NewManager(stateDb, contractDefDb)

	txDb, err := db.NewMDBXDatabase(mdbxEnv, "TRANSACTION")
	if err != nil {
		return err
	}
	receiptDb, err := db.NewMDBXDatabase(mdbxEnv, "RECEIPT")
	if err != nil {
		return err
	}
	n.transactionManager = transaction.NewManager(txDb, receiptDb)

	blockDb, err := db.NewMDBXDatabase(mdbxEnv, "BLOCK")
	if err != nil {
		return err
	}
	n.blockManager = block.NewManager(blockDb)

	n.feederClient, err = feeder.NewClient(n.cfg.Network.URL(), feederGatewaySuffix, nil, n.logger.Named("FEEDER"))
	if err != nil {
		return err
	}
	n.virtualMachine = cairovm.New(n.stateManager, n.logger.Named("VM"))
	n.synchronizer, err = syncer.NewSynchronizer(n.cfg.Network, n.cfg.EthNode, n.feederClient,
		n.syncManager, n.stateManager, n.blockManager, n.transactionManager, n.logger.Named("SYNC"))

	if err != nil {
		return err
	}

	rpcLogger := n.logger.Named("RPC")
	jsonRpc, err := starkNetJsonRPC(n.stateManager, n.blockManager, n.transactionManager,
		n.synchronizer, n.virtualMachine, rpcLogger)
	if err != nil {
		return err
	}
	rpcAddr := ":" + strconv.FormatUint(uint64(n.cfg.RpcPort), 10)
	n.rpcServer = rpc.NewHttpRpc(rpcAddr, rpcSuffix, jsonRpc, rpcLogger)

	if n.cfg.Metrics {
		n.metricsServer = prometheus.SetupMetric(defaultMetricsPort)
	}

	rpcErrCh := make(chan error)
	metricsErrCh := make(chan error)

	err = n.virtualMachine.Run(n.cfg.DatabasePath)
	if err != nil {
		return err
	}
	n.synchronizer.Run()
	n.rpcServer.ListenAndServe(rpcErrCh)

	if n.metricsServer != nil {
		n.metricsServer.ListenAndServe(metricsErrCh)
	}

	if err = <-rpcErrCh; err != nil {
		return err
	}

	if n.metricsServer != nil {
		if err = <-metricsErrCh; err != nil {
			return err
		}
	}

	return nil
}

func (n *Node) Shutdown() error {
	n.logger.Info("Shutting down Juno")
	n.virtualMachine.Close()
	n.synchronizer.Close()

	n.stateManager.Close()
	n.transactionManager.Close()
	n.blockManager.Close()
	n.syncManager.Close()

	if err := n.rpcServer.Close(shutdownTimeout); err != nil {
		return err
	}

	if n.metricsServer != nil {
		if err := n.metricsServer.Close(shutdownTimeout); err != nil {
			return err
		}
	}

	return nil
}

func starkNetJsonRPC(stateManager *state.Manager, blockManager *block.Manager,
	transactionManager *transaction.Manager, synchronizer *syncer.Synchronizer,
	virtualMachine *cairovm.VirtualMachine, logger log.Logger,
) (*jsonrpc.JsonRpc, error) {
	starkNetApi := starknet.New(stateManager, blockManager, transactionManager, synchronizer,
		virtualMachine, logger)
	jsonRpc := jsonrpc.NewJsonRpc()
	handlers := []struct {
		name       string
		function   any
		paramNames []string
	}{
		{"starknet_getBlockWithTxHashes", starkNetApi.GetBlockWithTxHashes, []string{"block_id"}},
		{"starknet_getBlockWithTxs", starkNetApi.GetBlockWithTxs, []string{"block_id"}},
		{"starknet_getStateUpdate", starkNetApi.GetStateUpdate, []string{"block_id"}},
		{"starknet_getStorageAt", starkNetApi.GetStorageAt, []string{"contract_address", "key", "block_id"}},
		{"starknet_getTransactionByHash", starkNetApi.GetTransactionByHash, []string{"transaction_hash"}},
		{"starknet_getTransactionByBlockIdAndIndex", starkNetApi.GetTransactionByBlockIdAndIndex, []string{"block_id", "index"}},
		{"starknet_getTransactionReceipt", starkNetApi.GetTransactionReceipt, []string{"transaction_hash"}},
		{"starknet_getClass", starkNetApi.GetClass, []string{"class_hash"}},
		{"starknet_getClassHashAt", starkNetApi.GetClassHashAt, []string{"block_id", "address"}},
		{"starknet_getBlockTransactionCount", starkNetApi.GetBlockTransactionCount, []string{"block_id"}},
		{"starknet_call", starkNetApi.Call, []string{"block_id", "request"}},
		{"starknet_estimateFee", starkNetApi.EstimateFee, []string{"block_id", "request"}},
		{"starknet_blockNumber", starkNetApi.BlockNumber, nil},
		{"starknet_blockHashAndNumber", starkNetApi.BlockHashAndNumber, nil},
		{"starknet_chainId", starkNetApi.ChainId, nil},
		{"starknet_pendingTransactions", starkNetApi.PendingTransactions, nil},
		{"starknet_protocolVersion", starkNetApi.ProtocolVersion, nil},
		{"starknet_syncing", starkNetApi.Syncing, nil},
	}
	for _, handler := range handlers {
		if err := jsonRpc.RegisterFunc(handler.name, handler.function, handler.paramNames...); err != nil {
			return nil, err
		}
	}
	return jsonRpc, nil
}
