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
	RPCCorsEnable       bool           `mapstructure:"rpc-cors-enable"`
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

	P2P           bool   `mapstructure:"p2p"`
	P2PAddr       string `mapstructure:"p2p-addr"`
	P2PPeers      string `mapstructure:"p2p-peers"`
	P2PFeederNode bool   `mapstructure:"p2p-feeder-node"`
	P2PPrivateKey string `mapstructure:"p2p-private-key"`

	MaxVMs          uint `mapstructure:"max-vms"`
	MaxVMQueue      uint `mapstructure:"max-vm-queue"`
	RPCMaxBlockScan uint `mapstructure:"rpc-max-block-scan"`
	RPCCallMaxSteps uint `mapstructure:"rpc-call-max-steps"`

	DBCacheSize  uint `mapstructure:"db-cache-size"`
	DBMaxHandles int  `mapstructure:"db-max-handles"`

	GatewayAPIKey  string        `mapstructure:"gw-api-key"`
	GatewayTimeout time.Duration `mapstructure:"gw-timeout"`
}

type Node struct {
	cfg        *Config
	db         db.DB
	blockchain *blockchain.Blockchain

	metricsService service.Service // Start the metrics service earlier than other services.
	services       []service.Service
	log            utils.Logger

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
		database, err = pebble.New(cfg.DatabasePath, cfg.DBCacheSize, cfg.DBMaxHandles, dbLog)
	}
	if err != nil {
		return nil, fmt.Errorf("open DB: %w", err)
	}
	ua := fmt.Sprintf("Juno/%s Starknet Client", version)

	services := make([]service.Service, 0)

	chain := blockchain.New(database, &cfg.Network)

	// Verify that cfg.Network is compatible with the database.
	head, err := chain.Head()
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, fmt.Errorf("get head block from database: %v", err)
	}
	if head != nil {
		// We assume that there is at least one transaction in the block or that it is a pre-0.7 block.
		if _, err = core.VerifyBlockHash(head, &cfg.Network); err != nil {
			return nil, errors.New("unable to verify latest block hash; are the database and --network option compatible?")
		}
	}

	client := feeder.NewClient(cfg.Network.FeederURL).WithUserAgent(ua).WithLogger(log).
		WithTimeout(cfg.GatewayTimeout).WithAPIKey(cfg.GatewayAPIKey)
	synchronizer := sync.New(chain, adaptfeeder.New(client), log, cfg.PendingPollInterval, dbIsRemote)
	gatewayClient := gateway.NewClient(cfg.Network.GatewayURL, log).WithUserAgent(ua).WithAPIKey(cfg.GatewayAPIKey)

	var p2pService *p2p.Service
	if cfg.P2P {
		if cfg.Network != utils.Sepolia {
			return nil, fmt.Errorf("P2P can only be used for %v network. Provided network: %v", utils.Sepolia, cfg.Network)
		}
		log.Warnw("P2P features enabled. Please note P2P is in experimental stage")

		if !cfg.P2PFeederNode {
			// Do not start the feeder synchronisation
			synchronizer = nil
		}
		p2pService, err = p2p.New(cfg.P2PAddr, "juno", cfg.P2PPeers, cfg.P2PPrivateKey, cfg.P2PFeederNode, chain, &cfg.Network, log)
		if err != nil {
			return nil, fmt.Errorf("set up p2p service: %w", err)
		}

		services = append(services, p2pService)
	}
	if synchronizer != nil {
		services = append(services, synchronizer)
	}

	throttledVM := NewThrottledVM(vm.New(log), cfg.MaxVMs, int32(cfg.MaxVMQueue))

	var syncReader sync.Reader = &sync.NoopSynchronizer{}
	if synchronizer != nil {
		syncReader = synchronizer
	}

	rpcHandler := rpc.New(chain, syncReader, throttledVM, version, &cfg.Network, log).WithGateway(gatewayClient).WithFeeder(client)
	rpcHandler = rpcHandler.WithFilterLimit(cfg.RPCMaxBlockScan).WithCallMaxSteps(uint64(cfg.RPCCallMaxSteps))
	services = append(services, rpcHandler)
	// to improve RPC throughput we double GOMAXPROCS
	maxGoroutines := 2 * runtime.GOMAXPROCS(0)
	jsonrpcServer := jsonrpc.NewServer(maxGoroutines, log).WithValidator(validator.Validator())
	methods, path := rpcHandler.Methods()
	if err = jsonrpcServer.RegisterMethods(methods...); err != nil {
		return nil, err
	}
	jsonrpcServerLegacy := jsonrpc.NewServer(maxGoroutines, log).WithValidator(validator.Validator())
	legacyMethods, legacyPath := rpcHandler.MethodsV0_6()
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
		services = append(services, makeRPCOverHTTP(cfg.HTTPHost, cfg.HTTPPort, rpcServers, log, cfg.Metrics, cfg.RPCCorsEnable))
	}
	if cfg.Websocket {
		services = append(services, makeRPCOverWebsocket(cfg.WebsocketHost, cfg.WebsocketPort, rpcServers, log, cfg.Metrics, cfg.RPCCorsEnable))
	}
	var metricsService service.Service
	if cfg.Metrics {
		makeJeMallocMetrics()
		makeVMThrottlerMetrics(throttledVM)
		makePebbleMetrics(database)
		chain.WithListener(makeBlockchainMetrics())
		makeJunoMetrics(version)
		database.WithListener(makeDBMetrics())
		rpcMetrics, legacyRPCMetrics := makeRPCMetrics(path, legacyPath)
		jsonrpcServer.WithListener(rpcMetrics)
		jsonrpcServerLegacy.WithListener(legacyRPCMetrics)
		client.WithListener(makeFeederMetrics())
		gatewayClient.WithListener(makeGatewayMetrics())
		metricsService = makeMetrics(cfg.MetricsHost, cfg.MetricsPort)

		if synchronizer != nil {
			synchronizer.WithListener(makeSyncMetrics(synchronizer, chain))
		} else if p2pService != nil {
			// regular p2p node
			p2pService.WithListener(makeSyncMetrics(&sync.NoopSynchronizer{}, chain))
		}
	}
	if cfg.GRPC {
		services = append(services, makeGRPC(cfg.GRPCHost, cfg.GRPCPort, database, version))
	}
	if cfg.Pprof {
		services = append(services, makePPROF(cfg.PprofHost, cfg.PprofPort))
	}

	n := &Node{
		cfg:            cfg,
		log:            log,
		version:        version,
		db:             database,
		blockchain:     chain,
		services:       services,
		metricsService: metricsService,
	}

	if n.cfg.EthNode == "" {
		n.log.Warnw("Ethereum node address not found; will not verify against L1")
	} else {
		var l1Client *l1.Client
		l1Client, err = newL1Client(cfg, n.blockchain, n.log)
		if err != nil {
			return nil, fmt.Errorf("create L1 client: %w", err)
		}
		n.services = append(n.services, l1Client)
	}

	if semversion, err := semver.NewVersion(version); err == nil {
		ug := upgrader.NewUpgrader(semversion, githubAPIUrl, latestReleaseURL, upgraderDelay, n.log)
		n.services = append(n.services, ug)
	} else {
		log.Warnw("Failed to parse Juno version, will not warn about new releases", "version", version)
	}

	return n, nil
}

func newL1Client(cfg *Config, chain *blockchain.Blockchain, log utils.SimpleLogger) (*l1.Client, error) {
	ethNodeURL, err := url.Parse(cfg.EthNode)
	if err != nil {
		return nil, fmt.Errorf("parse Ethereum node URL: %w", err)
	}
	if ethNodeURL.Scheme != "wss" && ethNodeURL.Scheme != "ws" {
		return nil, errors.New("non-websocket Ethereum node URL (need wss://... or ws://...): " + cfg.EthNode)
	}

	network := chain.Network()
	if err != nil {
		return nil, fmt.Errorf("find core contract address for network %s: %w", network.String(), err)
	}

	var ethSubscriber *l1.EthSubscriber
	ethSubscriber, err = l1.NewEthSubscriber(cfg.EthNode, network.CoreContractAddress)
	if err != nil {
		return nil, fmt.Errorf("set up ethSubscriber: %w", err)
	}

	l1Client, err := l1.NewClient(ethSubscriber, chain, log), nil
	if err != nil {
		return nil, fmt.Errorf("set up l1 client: %w", err)
	}

	if cfg.Metrics {
		l1Client.WithEventListener(makeL1Metrics())
	}
	return l1Client, nil
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

	wg := conc.NewWaitGroup()
	defer wg.Wait()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if n.metricsService != nil {
		wg.Go(func() {
			defer cancel()
			if err := n.metricsService.Run(ctx); err != nil {
				n.log.Errorw("Metrics error", "err", err)
			}
		})
	}

	if err := migration.MigrateIfNeeded(ctx, n.db, &n.cfg.Network, n.log); err != nil {
		if errors.Is(err, context.Canceled) {
			n.log.Infow("DB Migration cancelled")
			return
		}
		n.log.Errorw("Error while migrating the DB", "err", err)
		return
	}

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

	<-ctx.Done()
	n.log.Infow("Shutting down Juno...")
}

func (n *Node) Config() Config {
	return *n.cfg
}
