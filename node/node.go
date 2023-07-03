package node

import (
	"context"
	"fmt"
	"github.com/NethermindEth/juno/starknetdata"
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
	"github.com/sourcegraph/conc"
)

const (
	defaultPprofPort          = 9080
	l1BlockConfirmationPeriod = 16
)

// Config is the top-level juno configuration.
type Config struct {
	LogLevel            utils.LogLevel `mapstructure:"log-level"`
	RPCPort             uint16         `mapstructure:"rpc-port"`
	GRPCPort            uint16         `mapstructure:"grpc-port"`
	DatabasePath        string         `mapstructure:"db-path"`
	Network             utils.Network  `mapstructure:"network"`
	EthNode             string         `mapstructure:"eth-node"`
	Pprof               bool           `mapstructure:"pprof"`
	Colour              bool           `mapstructure:"colour"`
	PendingPollInterval time.Duration  `mapstructure:"pending-poll-interval"`

	P2P          bool   `mapstructure:"p2p"`
	P2PAddr      string `mapstructure:"p2pAddr"`
	P2PSync      bool   `mapstructure:"p2pSync"`
	P2PBootPeers string `mapstructure:"p2pBootPeers"`
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
	services := make([]service.Service, 0)

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
	var starkdata starknetdata.StarknetData
	starkdata = adaptfeeder.New(client)

	if cfg.P2P {
		p2pService, err := p2p.New(chain, cfg.P2PAddr, cfg.P2PBootPeers, log)
		if err != nil {
			log.Errorw("Error setting up p2p", "err", err)
			return nil, err
		}

		services = append(services, p2pService)

		if cfg.P2PSync {
			// TODO: Why is this complicated?
			blockSyncManager, poolService, err := p2pService.CreateBlockSyncProvider()
			if err != nil {
				log.Errorw("Error setting up p2p sync", "err", err)
				return nil, err
			}

			services = append(services, poolService)

			starkdata, err = p2p.NewStarknetDataAdapter(starkdata, blockSyncManager, chain, log)
			if err != nil {
				log.Errorw("Error adapting to p2p sync", "err", err)
				return nil, err
			}
		}
	}

	synchronizer := sync.New(chain, starkdata, log, cfg.PendingPollInterval)
	gatewayClient := gateway.NewClient(cfg.Network.GatewayURL(), log)
	http := makeHTTP(cfg.RPCPort, rpc.New(chain, synchronizer, cfg.Network, gatewayClient, version, log), log)

	services = append(services, synchronizer, http)

	n := &Node{
		cfg:        cfg,
		log:        log,
		version:    version,
		db:         database,
		blockchain: chain,
		services:   services,
	}

	if n.cfg.EthNode == "" {
		n.log.Warnw("Ethereum node address not found; will not verify against L1")
	} else {
		l1Client, err := l1.MakeClient(n.cfg.EthNode, n.blockchain, l1BlockConfirmationPeriod, n.log)
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

	return n, nil
}

/*
func (n *Node) setupP2P(ctx context.Context, starkdata starknetdata.StarknetData) (starknetdata.StarknetData, bool) {
	p2pClient, err := p2p.Start(n.blockchain, n.cfg.P2PAddr, n.cfg.P2PBootPeers, n.log)
	if err != nil {
		n.log.Errorw("Error starting p2p", "err", err)
		return nil, true
	}

	if !n.cfg.P2PSync {
		return starkdata, false
	}

	blockSyncProvider, err := p2pClient.CreateBlockSyncProvider(ctx)
	if err != nil {
		n.log.Errorw("Error adapting to p2p sync", "err", err)
		return nil, true
	}

	starkdata, err = p2p.NewStarknetDataAdapter(starkdata, blockSyncProvider, n.blockchain, n.log)
	if err != nil {
		n.log.Errorw("Error adapting to p2p sync", "err", err)
		return nil, true
	}

	return starkdata, false
}

*/

func makeHTTP(port uint16, rpcHandler *rpc.Handler, log utils.SimpleLogger) *jsonrpc.HTTP {
	return jsonrpc.NewHTTP(port, []jsonrpc.Method{
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
	}, log)
}

// Run starts Juno node by opening the DB, initialising services.
// All the services blocking and any errors returned by service run function is logged.
// Run will wait for all services to return before exiting.
func (n *Node) Run(ctx context.Context) {
	n.log.Infow("Starting Juno...", "config", fmt.Sprintf("%+v", *n.cfg))
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
