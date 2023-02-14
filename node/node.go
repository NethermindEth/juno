package node

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc"
	"github.com/NethermindEth/juno/starknetdata/gateway"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
)

type StarknetNode interface {
	Run(ctx context.Context) error
}

type NewStarknetNodeFn func(cfg *Config) (StarknetNode, error)

const (
	feederGatewaySuffix = "/feeder_gateway"
	rpcSuffix           = "/rpc"

	defaultMetricsPort = ":9090"

	shutdownTimeout = 5 * time.Second
)

// Config is the top-level juno configuration.
type Config struct {
	Verbosity    utils.LogLevel `mapstructure:"verbosity"`
	RpcPort      uint16         `mapstructure:"rpc-port"`
	Metrics      bool           `mapstructure:"metrics"`
	DatabasePath string         `mapstructure:"db-path"`
	Network      utils.Network  `mapstructure:"network"`
	EthNode      string         `mapstructure:"eth-node"`
}

type Node struct {
	cfg          *Config
	db           db.DB
	blockchain   *blockchain.Blockchain
	synchronizer *sync.Synchronizer
	http         *jsonrpc.Http
	verifier     *sync.L1Verifier

	log utils.Logger
}

func New(cfg *Config) (StarknetNode, error) {
	if !utils.IsValidNetwork(cfg.Network) {
		return nil, utils.ErrUnknownNetwork
	}
	if !cfg.Verbosity.IsValid() {
		return nil, utils.ErrUnknownLogLevel
	}
	if cfg.DatabasePath == "" {
		dirPrefix, err := utils.DefaultDataDir()
		if err != nil {
			return nil, err
		}
		cfg.DatabasePath = filepath.Join(dirPrefix, cfg.Network.String())
	}
	log, err := utils.NewZapLogger(cfg.Verbosity)
	if err != nil {
		return nil, err
	}
	dbLog, err := utils.NewZapLogger(utils.ERROR)
	if err != nil {
		return nil, err
	}
	stateDb, err := pebble.New(cfg.DatabasePath, dbLog)
	if err != nil {
		return nil, err
	}

	chain := blockchain.New(stateDb, cfg.Network)
	synchronizer := sync.NewSynchronizer(chain, gateway.NewGateway(cfg.Network), log)
	l1Verifier := sync.NewL1Verifier(chain, clients.NewGatewayClient(cfg.Network.URL()), log)
	return &Node{
		cfg:          cfg,
		log:          log,
		db:           stateDb,
		blockchain:   chain,
		verifier:     l1Verifier,
		synchronizer: synchronizer,
		http:         makeHttp(cfg.RpcPort, rpc.New(chain, cfg.Network.ChainId()), log),
	}, nil
}

func makeHttp(port uint16, rpcHandler *rpc.Handler, log utils.Logger) *jsonrpc.Http {
	return jsonrpc.NewHttp(port, []jsonrpc.Method{
		{"starknet_chainId", nil, rpcHandler.ChainId},
		{"starknet_blockNumber", nil, rpcHandler.BlockNumber},
		{"starknet_blockHashAndNumber", nil, rpcHandler.BlockNumberAndHash},
		{"starknet_getBlockWithTxHashes", []jsonrpc.Parameter{{Name: "block_id"}}, rpcHandler.GetBlockWithTxHashes},
		{"starknet_getBlockWithTxs", []jsonrpc.Parameter{{Name: "block_id"}}, rpcHandler.GetBlockWithTxs},
		{"starknet_getTransactionByHash", []jsonrpc.Parameter{{Name: "transaction_hash"}}, rpcHandler.GetTransactionByHash},
	}, log)
}

func (n *Node) Run(ctx context.Context) (err error) {
	n.log.Infow("Starting Juno...", "config", fmt.Sprintf("%+v", *n.cfg))
	defer func() {
		// Prioritise closing error over other errors
		if closeErr := n.db.Close(); closeErr != nil {
			err = closeErr
		}
	}()
	go func() {
		<-ctx.Done()
		n.log.Infow("Shutting down Juno...")
	}()
	n.http.Run(ctx)

	syncErrChan := make(chan error)
	verifierErrChan := make(chan error)

	go func() {
		syncErrChan <- n.synchronizer.Run(ctx)
	}()
	go func() {
		verifierErrChan <- n.verifier.Run(ctx)
	}()

	syncErr := <-syncErrChan
	verifierErr := <-verifierErrChan

	if syncErr != nil {
		return syncErr
	}
	if verifierErr != nil {
		return verifierErr
	}

	return err
}

func (n *Node) Config() Config {
	return *n.cfg
}
