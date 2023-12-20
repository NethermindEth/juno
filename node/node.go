package node

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"runtime"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/clients/gateway"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/db/remote"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/l1"
	"github.com/NethermindEth/juno/migration"
	"github.com/NethermindEth/juno/p2p"
	"github.com/NethermindEth/juno/rpc"
	"github.com/NethermindEth/juno/service"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/upgrader"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/validator"
	"github.com/NethermindEth/juno/vm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/mitchellh/mapstructure"
	"github.com/sourcegraph/conc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/yaml.v3"
)

const (
	upgraderDelay    = 5 * time.Minute
	githubAPIUrl     = "https://api.github.com/repos/NethermindEth/juno/releases/latest"
	latestReleaseURL = "https://github.com/NethermindEth/juno/releases/latest"
)

// Config is the top-level juno configuration.
type Config struct {
	LogLevel            utils.LogLevel `mapstructure:"log-level"`
	HTTP                bool           `mapstructure:"http"`
	HTTPHost            string         `mapstructure:"http-host"`
	HTTPPort            uint16         `mapstructure:"http-port"`
	Websocket           bool           `mapstructure:"ws"`
	WebsocketHost       string         `mapstructure:"ws-host"`
	WebsocketPort       uint16         `mapstructure:"ws-port"`
	GRPC                bool           `mapstructure:"grpc"`
	GRPCHost            string         `mapstructure:"grpc-host"`
	GRPCPort            uint16         `mapstructure:"grpc-port"`
	DatabasePath        string         `mapstructure:"db-path"`
	Network             utils.Network  `mapstructure:"network"`
	EthNode             string         `mapstructure:"eth-node"`
	Pprof               bool           `mapstructure:"pprof"`
	PprofHost           string         `mapstructure:"pprof-host"`
	PprofPort           uint16         `mapstructure:"pprof-port"`
	Colour              bool           `mapstructure:"colour"`
	PendingPollInterval time.Duration  `mapstructure:"pending-poll-interval"`
	RemoteDB            string         `mapstructure:"remote-db"`

	Metrics     bool   `mapstructure:"metrics"`
	MetricsHost string `mapstructure:"metrics-host"`
	MetricsPort uint16 `mapstructure:"metrics-port"`

	P2P          bool   `mapstructure:"p2p"`
	P2PAddr      string `mapstructure:"p2p-addr"`
	P2PBootPeers string `mapstructure:"p2p-boot-peers"`

	MaxVMs          uint `mapstructure:"max-vms"`
	MaxVMQueue      uint `mapstructure:"max-vm-queue"`
	RPCMaxBlockScan uint `mapstructure:"rpc-max-block-scan"`

	DBCacheSize uint `mapstructure:"db-cache-size"`

	GatewayAPIKey string `mapstructure:"gw-api-key"`
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
func New(cfg *Config, version string) (*Node, error) { //nolint:gocyclo,funlen
	log, err := utils.NewZapLogger(cfg.LogLevel, cfg.Colour)
	if err != nil {
		return nil, err
	}

	dbLog, err := utils.NewZapLogger(utils.ERROR, cfg.Colour)
	if err != nil {
		return nil, fmt.Errorf("create DB logger: %w", err)
	}

	dbIsRemote := cfg.RemoteDB != ""
	var database db.DB
	if dbIsRemote {
		database, err = remote.New(cfg.RemoteDB, context.TODO(), log, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		database, err = pebble.New(cfg.DatabasePath, cfg.DBCacheSize, dbLog)
	}
	if err != nil {
		return nil, fmt.Errorf("open DB: %w", err)
	}
	ua := fmt.Sprintf("Juno/%s Starknet Client", version)

	services := make([]service.Service, 0)

	chain := blockchain.New(database, cfg.Network, log)

	// Verify that cfg.Network is compatible with the database.
	head, err := chain.Head()
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, fmt.Errorf("get head block from database: %v", err)
	}
	if head != nil {
		// We assume that there is at least one transaction in the block or that it is a pre-0.7 block.
		if _, err = core.VerifyBlockHash(head, cfg.Network); err != nil {
			return nil, errors.New("unable to verify latest block hash; are the database and --network option compatible?")
		}
	}

	feederClientTimeout := 5 * time.Second
	client := feeder.NewClient(cfg.Network.FeederURL()).WithUserAgent(ua).WithLogger(log).
		WithTimeout(feederClientTimeout).WithAPIKey(cfg.GatewayAPIKey)
	synchronizer := sync.New(chain, adaptfeeder.New(client), log, cfg.PendingPollInterval, dbIsRemote)
	services = append(services, synchronizer)
	gatewayClient := gateway.NewClient(cfg.Network.GatewayURL(), log).WithUserAgent(ua).WithAPIKey(cfg.GatewayAPIKey)

	throttledVM := NewThrottledVM(vm.New(log), cfg.MaxVMs, int32(cfg.MaxVMQueue))
	rpcHandler := rpc.New(chain, synchronizer, cfg.Network, gatewayClient, client, throttledVM, version, log)
	rpcHandler = rpcHandler.WithFilterLimit(cfg.RPCMaxBlockScan)
	services = append(services, rpcHandler)
	// to improve RPC throughput we double GOMAXPROCS
	maxGoroutines := 2 * runtime.GOMAXPROCS(0)
	jsonrpcServer := jsonrpc.NewServer(maxGoroutines, log).WithValidator(validator.Validator())
	methods, path := rpcHandler.Methods()
	if err = jsonrpcServer.RegisterMethods(methods...); err != nil {
		return nil, err
	}
	jsonrpcServerLegacy := jsonrpc.NewServer(maxGoroutines, log).WithValidator(validator.Validator())
	legacyMethods, legacyPath := rpcHandler.LegacyMethods()
	if err = jsonrpcServerLegacy.RegisterMethods(legacyMethods...); err != nil {
		return nil, err
	}
	rpcServers := map[string]*jsonrpc.Server{
		"/":                 jsonrpcServer,
		path:                jsonrpcServer,
		legacyPath:          jsonrpcServerLegacy,
		"/rpc":              jsonrpcServer,
		"/rpc" + path:       jsonrpcServer,
		"/rpc" + legacyPath: jsonrpcServerLegacy,
	}
	if cfg.HTTP {
		services = append(services, makeRPCOverHTTP(cfg.HTTPHost, cfg.HTTPPort, rpcServers, log, cfg.Metrics))
	}
	if cfg.Websocket {
		services = append(services, makeRPCOverWebsocket(cfg.WebsocketHost, cfg.WebsocketPort, rpcServers, log, cfg.Metrics))
	}
	if cfg.Metrics {
		chain.WithListener(makeBlockchainMetrics())
		makeJunoMetrics(version)
		database.WithListener(makeDBMetrics())
		rpcMetrics, legacyRPCMetrics := makeRPCMetrics(path, legacyPath)
		jsonrpcServer.WithListener(rpcMetrics)
		jsonrpcServerLegacy.WithListener(legacyRPCMetrics)
		synchronizer.WithListener(makeSyncMetrics(synchronizer, chain))
		client.WithListener(makeFeederMetrics())
		services = append(services, makeMetrics(cfg.MetricsHost, cfg.MetricsPort))
	}
	if cfg.GRPC {
		services = append(services, makeGRPC(cfg.GRPCHost, cfg.GRPCPort, database, version))
	}
	if cfg.Pprof {
		services = append(services, makePPROF(cfg.PprofHost, cfg.PprofPort))
	}

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
		var ethNodeURL *url.URL
		ethNodeURL, err = url.Parse(n.cfg.EthNode)
		if err != nil {
			return nil, fmt.Errorf("parse Ethereum node URL: %w", err)
		}
		if ethNodeURL.Scheme != "wss" && ethNodeURL.Scheme != "ws" {
			return nil, errors.New("non-websocket Ethereum node URL (need wss://... or ws://...): " + n.cfg.EthNode)
		}
		var l1Client *l1.Client
		l1Client, err = newL1Client(n.cfg.EthNode, n.blockchain, n.log)
		if err != nil {
			return nil, fmt.Errorf("create L1 client: %w", err)
		}
		if cfg.Metrics {
			l1Client.WithEventListener(makeL1Metrics())
		}
		n.services = append(n.services, l1Client)
	}

	if cfg.P2P {
		p2pService, err := p2p.New(cfg.P2PAddr, "juno", cfg.P2PBootPeers, "", cfg.Network, log)
		if err != nil {
			return nil, fmt.Errorf("set up p2p service: %w", err)
		}

		n.services = append(n.services, p2pService)
	}

	if semversion, err := semver.NewVersion(version); err == nil {
		ug := upgrader.NewUpgrader(semversion, githubAPIUrl, latestReleaseURL, upgraderDelay, n.log)
		n.services = append(n.services, ug)
	} else {
		log.Warnw("Failed to parse Juno version, will not warn about new releases", "version", version)
	}

	return n, nil
}

func newL1Client(ethNode string, chain *blockchain.Blockchain, log utils.SimpleLogger) (*l1.Client, error) {
	var coreContractAddress common.Address
	coreContractAddress, err := chain.Network().CoreContractAddress()
	if err != nil {
		return nil, fmt.Errorf("find core contract address for network %s: %w", chain.Network(), err)
	}

	var ethSubscriber *l1.EthSubscriber
	ethSubscriber, err = l1.NewEthSubscriber(ethNode, coreContractAddress)
	if err != nil {
		return nil, fmt.Errorf("set up ethSubscriber: %w", err)
	}
	return l1.NewClient(ethSubscriber, chain, log), nil
}

// Run starts Juno node by opening the DB, initialising services.
// All the services blocking and any errors returned by service run function is logged.
// Run will wait for all services to return before exiting.
func (n *Node) Run(ctx context.Context) {
	defer func() {
		if closeErr := n.db.Close(); closeErr != nil {
			n.log.Errorw("Error while closing the DB", "err", closeErr)
		}
	}()

	cfg := make(map[string]interface{})
	err := mapstructure.Decode(n.cfg, &cfg)
	if err != nil {
		n.log.Errorw("Error while decoding config to mapstructure", "err", err)
		return
	}
	yamlConfig, err := yaml.Marshal(n.cfg)
	if err != nil {
		n.log.Errorw("Error while marshalling config", "err", err)
		return
	}
	n.log.Debugw(fmt.Sprintf("Running Juno with config:\n%s", string(yamlConfig)))

	if err := migration.MigrateIfNeeded(ctx, n.db, n.cfg.Network, n.log); err != nil {
		if errors.Is(err, context.Canceled) {
			n.log.Infow("DB Migration cancelled")
			return
		}
		n.log.Errorw("Error while migrating the DB", "err", err)
		return
	}

	ctx, cancel := context.WithCancel(ctx)
	wg := conc.NewWaitGroup()
	for _, s := range n.services {
		s := s
		wg.Go(func() {
			// Immediately acknowledge panicing services by shutting down the node
			// Without the deffered cancel(), we would have to wait for user to hit Ctrl+C
			defer cancel()
			if err := s.Run(ctx); err != nil {
				n.log.Errorw("Service error", "name", reflect.TypeOf(s), "err", err)
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
