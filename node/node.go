package node

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/clients/gateway"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/grpc"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/l1"
	"github.com/NethermindEth/juno/migration"
	"github.com/NethermindEth/juno/p2p"
	"github.com/NethermindEth/juno/pprof"
	"github.com/NethermindEth/juno/rpc"
	"github.com/NethermindEth/juno/service"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/validator"
	"github.com/ethereum/go-ethereum/common"
	"github.com/sourcegraph/conc"
)

const defaultPprofPort = 9080

// Config is the top-level juno configuration.
type Config struct {
	LogLevel            utils.LogLevel `mapstructure:"log-level"`
	HTTPPort            uint16         `mapstructure:"http-port"`
	WSPort              uint16         `mapstructure:"ws-port"`
	GRPCPort            uint16         `mapstructure:"grpc-port"`
	DatabasePath        string         `mapstructure:"db-path"`
	Network             utils.Network  `mapstructure:"network"`
	EthNode             string         `mapstructure:"eth-node"`
	Pprof               bool           `mapstructure:"pprof"`
	Colour              bool           `mapstructure:"colour"`
	PendingPollInterval time.Duration  `mapstructure:"pending-poll-interval"`

	P2P          bool   `mapstructure:"p2p"`
	P2PAddr      string `mapstructure:"p2p-addr"`
	P2PBootPeers string `mapstructure:"p2p-boot-peers"`
}

type Node struct {
	cfg        *Config
	db         db.DB
	blockchain *blockchain.Blockchain

	services []service.Service
	log      utils.Logger

	version string
}

// New sets the config and logger to the StarknetNode.
// Any errors while parsing the config on creating logger will be returned.
func New(cfg *Config, version string) (*Node, error) {
	if cfg.DatabasePath == "" {
		dirPrefix, err := utils.DefaultDataDir()
		if err != nil {
			return nil, err
		}
		cfg.DatabasePath = filepath.Join(dirPrefix, cfg.Network.String())
	}
	log, err := utils.NewZapLogger(cfg.LogLevel, cfg.Colour)
	if err != nil {
		return nil, err
	}

	dbLog, err := utils.NewZapLogger(utils.ERROR, cfg.Colour)
	if err != nil {
		log.Errorw("Error creating DB logger", "err", err)
		return nil, err
	}

	database, err := pebble.New(cfg.DatabasePath, dbLog)
	if err != nil {
		log.Errorw("Error opening DB", "err", err)
		return nil, err
	}

	chain := blockchain.New(database, cfg.Network, log)
	client := feeder.NewClient(cfg.Network.FeederURL())

	synchronizer := sync.New(chain, adaptfeeder.New(client), log, cfg.PendingPollInterval)
	gatewayClient := gateway.NewClient(cfg.Network.GatewayURL(), log)

	services, err := makeRPC(cfg.HTTPPort, cfg.WSPort, rpc.New(chain, synchronizer, cfg.Network, gatewayClient, client, version, log), log)
	if err != nil {
		log.Errorw("Failed to create RPC servers", "err", err)
		return nil, err
	}

	n := &Node{
		cfg:        cfg,
		log:        log,
		version:    version,
		db:         database,
		blockchain: chain,
		services:   append(services, synchronizer),
	}

	if n.cfg.EthNode == "" {
		n.log.Warnw("Ethereum node address not found; will not verify against L1")
	} else {
		l1Client, err := newL1Client(n.cfg.EthNode, n.blockchain, n.log)
		if err != nil {
			n.log.Errorw("Error creating L1 client", "err", err)
			return nil, err
		}

		n.services = append(n.services, l1Client)
	}

	if n.cfg.Pprof {
		n.services = append(n.services, pprof.New(defaultPprofPort, n.log))
	}

	if n.cfg.GRPCPort > 0 {
		n.services = append(n.services, grpc.NewServer(n.cfg.GRPCPort, n.version, n.db, n.log))
	}

	if cfg.P2P {
		privKeyStr, _ := os.LookupEnv("P2P_PRIVATE_KEY")
		p2pService, err := p2p.New(cfg.P2PAddr, "juno", cfg.P2PBootPeers, privKeyStr, chain, cfg.Network, log)
		if err != nil {
			log.Errorw("Error setting up p2p", "err", err)
			return nil, err
		}

		n.services = append(n.services, p2pService)
	}

	return n, nil
}

func makeRPC(httpPort, wsPort uint16, rpcHandler *rpc.Handler, log utils.SimpleLogger) ([]service.Service, error) { //nolint: funlen
	methods := []jsonrpc.Method{
		{
			Name:    "starknet_chainId",
			Handler: rpcHandler.ChainID,
		},
		{
			Name:    "starknet_blockNumber",
			Handler: rpcHandler.BlockNumber,
		},
		{
			Name:    "starknet_blockHashAndNumber",
			Handler: rpcHandler.BlockHashAndNumber,
		},
		{
			Name:    "starknet_getBlockWithTxHashes",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: rpcHandler.BlockWithTxHashes,
		},
		{
			Name:    "starknet_getBlockWithTxs",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: rpcHandler.BlockWithTxs,
		},
		{
			Name:    "starknet_getTransactionByHash",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: rpcHandler.TransactionByHash,
		},
		{
			Name:    "starknet_getTransactionReceipt",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: rpcHandler.TransactionReceiptByHash,
		},
		{
			Name:    "starknet_getBlockTransactionCount",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: rpcHandler.BlockTransactionCount,
		},
		{
			Name:    "starknet_getTransactionByBlockIdAndIndex",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "index"}},
			Handler: rpcHandler.TransactionByBlockIDAndIndex,
		},
		{
			Name:    "starknet_getStateUpdate",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: rpcHandler.StateUpdate,
		},
		{
			Name:    "starknet_syncing",
			Handler: rpcHandler.Syncing,
		},
		{
			Name:    "starknet_getNonce",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: rpcHandler.Nonce,
		},
		{
			Name:    "starknet_getStorageAt",
			Params:  []jsonrpc.Parameter{{Name: "contract_address"}, {Name: "key"}, {Name: "block_id"}},
			Handler: rpcHandler.StorageAt,
		},
		{
			Name:    "starknet_getClassHashAt",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: rpcHandler.ClassHashAt,
		},
		{
			Name:    "starknet_getClass",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "class_hash"}},
			Handler: rpcHandler.Class,
		},
		{
			Name:    "starknet_getClassAt",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: rpcHandler.ClassAt,
		},
		{
			Name:    "starknet_addInvokeTransaction",
			Params:  []jsonrpc.Parameter{{Name: "invoke_transaction"}},
			Handler: rpcHandler.AddInvokeTransaction,
		},
		{
			Name:    "starknet_addDeployAccountTransaction",
			Params:  []jsonrpc.Parameter{{Name: "deploy_account_transaction"}},
			Handler: rpcHandler.AddDeployAccountTransaction,
		},
		{
			Name:    "starknet_addDeclareTransaction",
			Params:  []jsonrpc.Parameter{{Name: "declare_transaction"}},
			Handler: rpcHandler.AddDeclareTransaction,
		},
		{
			Name:    "starknet_getEvents",
			Params:  []jsonrpc.Parameter{{Name: "filter"}},
			Handler: rpcHandler.Events,
		},
		{
			Name:    "starknet_pendingTransactions",
			Handler: rpcHandler.PendingTransactions,
		},
		{
			Name:    "juno_version",
			Handler: rpcHandler.Version,
		},
		{
			Name:    "juno_getTransactionStatus",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: rpcHandler.TransactionStatus,
		},
		{
			Name:    "starknet_call",
			Params:  []jsonrpc.Parameter{{Name: "request"}, {Name: "block_id"}},
			Handler: rpcHandler.Call,
		},
		{
			Name:    "starknet_estimateFee",
			Params:  []jsonrpc.Parameter{{Name: "request"}, {Name: "block_id"}},
			Handler: rpcHandler.EstimateFee,
		},
	}

	jsonrpcServer := jsonrpc.NewServer(log).WithValidator(validator.Validator())
	for _, method := range methods {
		if err := jsonrpcServer.RegisterMethod(method); err != nil {
			return nil, err
		}
	}

	httpListener, err := net.Listen("tcp", fmt.Sprintf(":%d", httpPort))
	if err != nil {
		return nil, fmt.Errorf("listen on http port %d: %w", httpPort, err)
	}
	httpServer := jsonrpc.NewHTTP(httpListener, jsonrpcServer, log)

	wsListener, err := net.Listen("tcp", fmt.Sprintf(":%d", wsPort))
	if err != nil {
		return nil, fmt.Errorf("listen on websocket port %d: %w", wsPort, err)
	}
	wsServer := jsonrpc.NewWebsocket(wsListener, jsonrpcServer, log)

	return []service.Service{httpServer, wsServer}, nil
}

func newL1Client(ethNode string, chain *blockchain.Blockchain, log utils.SimpleLogger) (*l1.Client, error) {
	var coreContractAddress common.Address
	coreContractAddress, err := chain.Network().CoreContractAddress()
	if err != nil {
		log.Errorw("Error finding core contract address for network", "err", err, "network", chain.Network())
		return nil, err
	}

	var ethSubscriber *l1.EthSubscriber
	ethSubscriber, err = l1.NewEthSubscriber(ethNode, coreContractAddress)
	if err != nil {
		log.Errorw("Error creating ethSubscriber", "err", err)
		return nil, err
	}
	return l1.NewClient(ethSubscriber, chain, log), nil
}

// Run starts Juno node by opening the DB, initialising services.
// All the services blocking and any errors returned by service run function is logged.
// Run will wait for all services to return before exiting.
func (n *Node) Run(ctx context.Context) {
	n.log.Infow("Starting Juno...", "config", fmt.Sprintf("%+v", *n.cfg), "version", n.version)
	defer func() {
		if closeErr := n.db.Close(); closeErr != nil {
			n.log.Errorw("Error while closing the DB", "err", closeErr)
		}
	}()

	if err := migration.MigrateIfNeeded(n.db); err != nil {
		n.log.Errorw("Error while migrating the DB", "err", err)
		return
	}

	ctx, cancel := context.WithCancel(ctx)
	wg := conc.NewWaitGroup()
	for _, s := range n.services {
		s := s
		wg.Go(func() {
			if err := s.Run(ctx); err != nil {
				n.log.Errorw("Service error", "name", reflect.TypeOf(s), "err", err)
				cancel()
			}
		})
	}
	defer wg.Wait()

	<-ctx.Done()
	cancel()
	n.log.Infow("Shutting down Juno...")
}

func (n *Node) Config() Config {
	return *n.cfg
}
