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
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/internal/metrics/prometheus"
	"github.com/NethermindEth/juno/internal/rpc"
	"github.com/NethermindEth/juno/internal/rpc/starknet"
	syncer "github.com/NethermindEth/juno/internal/sync"
	"github.com/NethermindEth/juno/internal/utils"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/NethermindEth/juno/pkg/jsonrpc"
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
	log.Logger.Info("Running Juno with config: ", fmt.Sprintf("%+v", *n.cfg))

	if err := utils.CreateDir(n.cfg.DatabasePath); err != nil {
		return err
	}

	if err := log.SetGlobalLogger(n.cfg.Verbosity); err != nil {
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

	n.feederClient = feeder.NewClient(n.cfg.Network.URL(), feederGatewaySuffix, nil)
	n.virtualMachine = cairovm.New(n.stateManager)
	n.synchronizer = syncer.NewSynchronizer(n.cfg.Network, n.cfg.EthNode, n.feederClient,
		n.syncManager, n.stateManager, n.blockManager, n.transactionManager)

	jsonRpc, err := starkNetJsonRPC(n.stateManager, n.blockManager, n.transactionManager,
		n.synchronizer, n.virtualMachine)
	if err != nil {
		return err
	}
	rpcAddr := ":" + strconv.FormatUint(uint64(n.cfg.RpcPort), 10)
	n.rpcServer, err = rpc.NewHttpRpc(rpcAddr, rpcSuffix, jsonRpc)
	if err != nil {
		return err
	}

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
	n.stateManager.Close()
	n.transactionManager.Close()
	n.blockManager.Close()
	n.stateManager.Close()
	n.synchronizer.Close()
	n.virtualMachine.Close()

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
	virtualMachine *cairovm.VirtualMachine,
) (*jsonrpc.JsonRpc, error) {
	starkNetApi := starknet.New(stateManager, blockManager, transactionManager, synchronizer,
		virtualMachine)
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
