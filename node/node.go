package node

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"runtime"
	"slices"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/clients/gateway"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/db/remote"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/l1"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/migration"
	"github.com/NethermindEth/juno/p2p"
	"github.com/NethermindEth/juno/plugin"
	"github.com/NethermindEth/juno/rpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/sequencer"
	"github.com/NethermindEth/juno/service"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/upgrader"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/validator"
	"github.com/NethermindEth/juno/vm"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/ecdsa"
	"github.com/mitchellh/mapstructure"
	"github.com/sourcegraph/conc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/yaml.v3"
)

const (
	upgraderDelay    = 5 * time.Minute
	mempoolLimit     = 1024
	githubAPIUrl     = "https://api.github.com/repos/NethermindEth/juno/releases/latest"
	latestReleaseURL = "https://github.com/NethermindEth/juno/releases/latest"
	sequencerAddress = 1337
)

// Config is the top-level juno configuration.
type Config struct {
	LogLevel                 string        `mapstructure:"log-level"`
	HTTP                     bool          `mapstructure:"http"`
	HTTPHost                 string        `mapstructure:"http-host"`
	HTTPPort                 uint16        `mapstructure:"http-port"`
	RPCCorsEnable            bool          `mapstructure:"rpc-cors-enable"`
	Websocket                bool          `mapstructure:"ws"`
	WebsocketHost            string        `mapstructure:"ws-host"`
	WebsocketPort            uint16        `mapstructure:"ws-port"`
	GRPC                     bool          `mapstructure:"grpc"`
	GRPCHost                 string        `mapstructure:"grpc-host"`
	GRPCPort                 uint16        `mapstructure:"grpc-port"`
	DatabasePath             string        `mapstructure:"db-path"`
	Network                  utils.Network `mapstructure:"network"`
	EthNode                  string        `mapstructure:"eth-node"`
	DisableL1Verification    bool          `mapstructure:"disable-l1-verification"`
	Pprof                    bool          `mapstructure:"pprof"`
	PprofHost                string        `mapstructure:"pprof-host"`
	PprofPort                uint16        `mapstructure:"pprof-port"`
	Colour                   bool          `mapstructure:"colour"`
	PendingPollInterval      time.Duration `mapstructure:"pending-poll-interval"`
	PreConfirmedPollInterval time.Duration `mapstructure:"preconfirmed-poll-interval"`
	RemoteDB                 string        `mapstructure:"remote-db"`
	VersionedConstantsFile   string        `mapstructure:"versioned-constants-file"`

	Sequencer      bool   `mapstructure:"seq-enable"`
	SeqBlockTime   uint   `mapstructure:"seq-block-time"`
	SeqGenesisFile string `mapstructure:"seq-genesis-file"`
	SeqDisableFees bool   `mapstructure:"seq-disable-fees"`

	Metrics     bool   `mapstructure:"metrics"`
	MetricsHost string `mapstructure:"metrics-host"`
	MetricsPort uint16 `mapstructure:"metrics-port"`

	P2P           bool   `mapstructure:"p2p"`
	P2PAddr       string `mapstructure:"p2p-addr"`
	P2PPublicAddr string `mapstructure:"p2p-public-addr"`
	P2PPeers      string `mapstructure:"p2p-peers"`
	P2PFeederNode bool   `mapstructure:"p2p-feeder-node"`
	P2PPrivateKey string `mapstructure:"p2p-private-key"`

	MaxVMs                  uint   `mapstructure:"max-vms"`
	MaxVMQueue              uint   `mapstructure:"max-vm-queue"`
	RPCMaxBlockScan         uint   `mapstructure:"rpc-max-block-scan"`
	RPCCallMaxSteps         uint64 `mapstructure:"rpc-call-max-steps"`
	RPCCallMaxGas           uint64 `mapstructure:"rpc-call-max-gas"`
	ReadinessBlockTolerance uint   `mapstructure:"readiness-block-tolerance"`

	SubmittedTransactionsCacheSize     uint          `mapstructure:"submitted-transactions-cache-size"`
	SubmittedTransactionsCacheEntryTTL time.Duration `mapstructure:"submitted-transactions-cache-entry-ttl"`

	DBCacheSize  uint `mapstructure:"db-cache-size"`
	DBMaxHandles int  `mapstructure:"db-max-handles"`

	GatewayAPIKey   string `mapstructure:"gw-api-key"`
	GatewayTimeouts string `mapstructure:"gw-timeouts"`

	PluginPath string `mapstructure:"plugin-path"`

	HTTPUpdateHost string `mapstructure:"http-update-host"`
	HTTPUpdatePort uint16 `mapstructure:"http-update-port"`

	ForbidRPCBatchRequests bool `mapstructure:"disable-rpc-batch-requests"`

	NewState bool `mapstructure:"new-state"`
}

type Node struct {
	cfg        *Config
	db         db.KeyValueStore
	blockchain *blockchain.Blockchain

	earlyServices []service.Service // Services that needs to start before than other services and before migration.
	services      []service.Service
	log           utils.Logger

	version string
}

// New sets the config and logger to the StarknetNode.
// Any errors while parsing the config on creating logger will be returned.
// Todo: (immediate follow-up PR) tidy this function up.
func New(cfg *Config, version string, logLevel *utils.LogLevel) (*Node, error) { //nolint:gocyclo,funlen
	log, err := utils.NewZapLogger(logLevel, cfg.Colour)
	if err != nil {
		return nil, err
	}

	dbIsRemote := cfg.RemoteDB != ""
	var database db.KeyValueStore
	if dbIsRemote {
		database, err = remote.New(cfg.RemoteDB, context.TODO(), log, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		database, err = pebble.New(
			cfg.DatabasePath,
			pebble.WithCacheSize(cfg.DBCacheSize),
			pebble.WithMaxOpenFiles(cfg.DBMaxHandles),
			pebble.WithLogger(cfg.Colour),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("open DB: %w", err)
	}
	ua := fmt.Sprintf("Juno/%s Starknet Client", version)

	services := make([]service.Service, 0)
	earlyServices := make([]service.Service, 0)

	chain := blockchain.New(database, &cfg.Network, cfg.NewState)

	// Verify that cfg.Network is compatible with the database.
	head, err := chain.Head()
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, fmt.Errorf("get head block from database: %v", err)
	}
	if head != nil {
		stateUpdate, err := chain.StateUpdateByNumber(head.Number)
		if err != nil {
			return nil, err
		}

		// We assume that there is at least one transaction in the block or that it is a pre-0.7 block.
		if _, err = core.VerifyBlockHash(head, &cfg.Network, stateUpdate.StateDiff); err != nil {
			return nil, errors.New("unable to verify latest block hash; are the database and --network option compatible?")
		}
	}

	if cfg.VersionedConstantsFile != "" {
		err = vm.SetVersionedConstants(cfg.VersionedConstantsFile)
		if err != nil {
			return nil, fmt.Errorf("failed to set versioned constants: %w", err)
		}
	}

	var synchronizer *sync.Synchronizer
	var rpcHandler *rpc.Handler
	var client *feeder.Client
	var gatewayClient *gateway.Client
	var p2pService *p2p.Service

	var junoPlugin plugin.JunoPlugin
	if cfg.PluginPath != "" {
		junoPlugin, err = plugin.Load(cfg.PluginPath)
		if err != nil {
			return nil, err
		}
		services = append(services, plugin.NewService(junoPlugin))
	}

	var nodeVM vm.VM
	var throttledVM *ThrottledVM

	if cfg.Sequencer {
		// Sequencer mode only supports known networks and
		// uses default fee tokens (custom networks not supported yet)
		if !slices.Contains(utils.KnownNetworkNames, cfg.Network.Name) {
			return nil, fmt.Errorf("custom networks are not supported in sequencer mode yet")
		}
		pKey, kErr := ecdsa.GenerateKey(rand.Reader) // Todo: currently private key changes with every sequencer run
		if kErr != nil {
			return nil, kErr
		}
		feeTokens := utils.DefaultFeeTokenAddresses
		chainInfo := vm.ChainInfo{
			ChainID:           cfg.Network.L2ChainID,
			FeeTokenAddresses: feeTokens,
		}
		nodeVM = vm.New(&chainInfo, false, log)
		throttledVM = NewThrottledVM(nodeVM, cfg.MaxVMs, int32(cfg.MaxVMQueue))
		mempool := mempool.New(database, chain, mempoolLimit, log)
		executor := builder.NewExecutor(chain, nodeVM, log, cfg.SeqDisableFees, false)
		builder := builder.New(chain, executor)
		seq := sequencer.New(&builder, mempool, new(felt.Felt).SetUint64(sequencerAddress),
			pKey, time.Second*time.Duration(cfg.SeqBlockTime), log)
		seq.WithPlugin(junoPlugin)
		rpcHandler = rpc.New(chain, &seq, throttledVM, version, log, &cfg.Network).
			WithMempool(mempool).
			WithCallMaxSteps(cfg.RPCCallMaxSteps).
			WithCallMaxGas(cfg.RPCCallMaxGas)
		services = append(services, &seq)
	} else {
		if cfg.GatewayTimeouts == "" {
			cfg.GatewayTimeouts = feeder.DefaultTimeouts
		}
		timeouts, fixed, err := feeder.ParseTimeouts(cfg.GatewayTimeouts)
		if err != nil {
			return nil, fmt.Errorf("invalid gateway timeouts: %w", err)
		}
		client = feeder.NewClient(cfg.Network.FeederURL).
			WithUserAgent(ua).
			WithLogger(log).
			WithTimeouts(timeouts, fixed).
			WithAPIKey(cfg.GatewayAPIKey)

		// Handle fee tokens for custom networks
		feeTokens := utils.DefaultFeeTokenAddresses
		if !slices.Contains(utils.KnownNetworkNames, cfg.Network.Name) {
			// For custom networks, fetch fee tokens from the gateway
			feeTokens, err = client.FeeTokenAddresses(context.Background())
			if err != nil {
				return nil, fmt.Errorf("failed to fetch fee token addresses for custom network: %w", err)
			}
		}
		chainInfo := vm.ChainInfo{
			ChainID:           cfg.Network.L2ChainID,
			FeeTokenAddresses: feeTokens,
		}
		nodeVM = vm.New(&chainInfo, false, log)
		throttledVM = NewThrottledVM(nodeVM, cfg.MaxVMs, int32(cfg.MaxVMQueue))
		feederGatewayDataSource := sync.NewFeederGatewayDataSource(chain, adaptfeeder.New(client))
		synchronizer = sync.New(
			chain,
			feederGatewayDataSource,
			log,
			cfg.PendingPollInterval,
			cfg.PreConfirmedPollInterval,
			dbIsRemote,
			database,
		)
		synchronizer.WithPlugin(junoPlugin)

		gatewayClient = gateway.NewClient(cfg.Network.GatewayURL, log).WithUserAgent(ua).WithAPIKey(cfg.GatewayAPIKey)

		if cfg.P2P {
			if cfg.Network == utils.Mainnet {
				return nil, fmt.Errorf("P2P cannot be used on %v network", utils.Mainnet)
			}
			log.Warnw("P2P features enabled. Please note P2P is in experimental stage")

			if !cfg.P2PFeederNode {
				// Do not start the feeder synchronisation
				synchronizer = nil
			}
			p2pService, err = p2p.New(cfg.P2PAddr, cfg.P2PPublicAddr, version, cfg.P2PPeers, cfg.P2PPrivateKey, cfg.P2PFeederNode,
				chain, &cfg.Network, log, database)
			if err != nil {
				return nil, fmt.Errorf("set up p2p service: %w", err)
			}

			services = append(services, p2pService)
		}

		var syncReader sync.Reader = &sync.NoopSynchronizer{}
		if synchronizer != nil {
			syncReader = synchronizer
		}

		submittedTransactionsCache := rpccore.NewTransactionCache(cfg.SubmittedTransactionsCacheEntryTTL, cfg.SubmittedTransactionsCacheSize)
		services = append(services, submittedTransactionsCache)
		rpcHandler = rpc.New(chain, syncReader, throttledVM, version, log, &cfg.Network).
			WithGateway(gatewayClient).
			WithFeeder(client).
			WithSubmittedTransactionsCache(submittedTransactionsCache).
			WithFilterLimit(cfg.RPCMaxBlockScan).
			WithCallMaxSteps(cfg.RPCCallMaxSteps).
			WithCallMaxGas(cfg.RPCCallMaxGas)
		if synchronizer != nil {
			services = append(services, synchronizer)
		}
	}

	services = append(services, rpcHandler)

	// to improve RPC throughput we double GOMAXPROCS
	maxGoroutines := 2 * runtime.GOMAXPROCS(0)

	jsonrpcServerV10 := jsonrpc.NewServer(maxGoroutines, log).
		WithValidator(validator.Validator()).
		DisableBatchRequests(cfg.ForbidRPCBatchRequests)
	methodsV10, pathV10 := rpcHandler.MethodsV0_10()
	if err = jsonrpcServerV10.RegisterMethods(methodsV10...); err != nil {
		return nil, err
	}

	jsonrpcServerV09 := jsonrpc.NewServer(maxGoroutines, log).
		WithValidator(validator.Validator()).
		DisableBatchRequests(cfg.ForbidRPCBatchRequests)
	methodsV09, pathV09 := rpcHandler.MethodsV0_9()
	if err = jsonrpcServerV09.RegisterMethods(methodsV09...); err != nil {
		return nil, err
	}

	jsonrpcServerV08 := jsonrpc.NewServer(maxGoroutines, log).
		WithValidator(validator.Validator()).
		DisableBatchRequests(cfg.ForbidRPCBatchRequests)
	methodsV08, pathV08 := rpcHandler.MethodsV0_8()
	if err = jsonrpcServerV08.RegisterMethods(methodsV08...); err != nil {
		return nil, err
	}

	jsonrpcServerV07 := jsonrpc.NewServer(maxGoroutines, log).
		DisableBatchRequests(cfg.ForbidRPCBatchRequests)
	methodsV07, pathV07 := rpcHandler.MethodsV0_7()
	if err = jsonrpcServerV07.RegisterMethods(methodsV07...); err != nil {
		return nil, err
	}

	jsonrpcServerV06 := jsonrpc.NewServer(maxGoroutines, log).
		DisableBatchRequests(cfg.ForbidRPCBatchRequests)
	methodsV06, pathV06 := rpcHandler.MethodsV0_6()
	if err = jsonrpcServerV06.RegisterMethods(methodsV06...); err != nil {
		return nil, err
	}

	rpcServers := map[string]*jsonrpc.Server{
		"/":              jsonrpcServerV08,
		pathV10:          jsonrpcServerV10,
		pathV09:          jsonrpcServerV09,
		pathV08:          jsonrpcServerV08,
		pathV07:          jsonrpcServerV07,
		pathV06:          jsonrpcServerV06,
		"/rpc":           jsonrpcServerV08,
		"/rpc" + pathV10: jsonrpcServerV10,
		"/rpc" + pathV09: jsonrpcServerV09,
		"/rpc" + pathV08: jsonrpcServerV08,
		"/rpc" + pathV07: jsonrpcServerV07,
		"/rpc" + pathV06: jsonrpcServerV06,
	}
	if cfg.HTTP {
		readinessHandlers := NewReadinessHandlers(chain, synchronizer, cfg.ReadinessBlockTolerance)
		httpHandlers := map[string]http.HandlerFunc{
			"/live":       readinessHandlers.HandleLive,
			"/ready":      readinessHandlers.HandleReadySync,
			"/ready/sync": readinessHandlers.HandleReadySync,
		}
		services = append(services, makeRPCOverHTTP(cfg.HTTPHost, cfg.HTTPPort, rpcServers, httpHandlers, log, cfg.Metrics, cfg.RPCCorsEnable))
	}
	if cfg.Websocket {
		services = append(services,
			makeRPCOverWebsocket(cfg.WebsocketHost, cfg.WebsocketPort, rpcServers, log, cfg.Metrics, cfg.RPCCorsEnable))
	}
	if cfg.HTTPUpdatePort != 0 {
		log.Infow("Log level and feeder gateway timeouts can be changed via HTTP PUT request to " +
			cfg.HTTPUpdateHost + ":" + fmt.Sprintf("%d", cfg.HTTPUpdatePort) + "/log/level and /feeder/timeouts",
		)
		earlyServices = append(earlyServices, makeHTTPUpdateService(cfg.HTTPUpdateHost, cfg.HTTPUpdatePort, logLevel, client))
	}
	if cfg.Metrics {
		makeJeMallocMetrics()
		makeVMThrottlerMetrics(throttledVM)
		makePebbleMetrics(database)
		chain.WithListener(makeBlockchainMetrics())
		makeJunoMetrics(version)
		database.WithListener(makeDBMetrics())
		rpcMetrics := makeRPCMetrics(pathV10, pathV09, pathV08, pathV07, pathV06)
		jsonrpcServerV10.WithListener(rpcMetrics[0])
		jsonrpcServerV09.WithListener(rpcMetrics[1])
		jsonrpcServerV08.WithListener(rpcMetrics[2])
		jsonrpcServerV07.WithListener(rpcMetrics[3])
		jsonrpcServerV06.WithListener(rpcMetrics[4])
		if !cfg.Sequencer {
			client.WithListener(makeFeederMetrics())
			gatewayClient.WithListener(makeGatewayMetrics())
			if synchronizer != nil {
				synchronizer.WithListener(makeSyncMetrics(synchronizer, chain))
			} else if p2pService != nil {
				// regular p2p node
				p2pService.WithListener(makeSyncMetrics(&sync.NoopSynchronizer{}, chain))
				p2pService.WithGossipTracer()
			}
		}
		earlyServices = append(earlyServices, makeMetrics(cfg.MetricsHost, cfg.MetricsPort))
	}
	if cfg.GRPC {
		services = append(services, makeGRPC(cfg.GRPCHost, cfg.GRPCPort, database, version))
	}
	if cfg.Pprof {
		services = append(services, makePPROF(cfg.PprofHost, cfg.PprofPort))
	}

	n := &Node{
		cfg:           cfg,
		log:           log,
		version:       version,
		db:            database,
		blockchain:    chain,
		services:      services,
		earlyServices: earlyServices,
	}

	if !n.cfg.DisableL1Verification {
		// Due to mutually exclusive flag we can do the following.
		if n.cfg.EthNode == "" {
			return nil, fmt.Errorf("ethereum node address not found; Use --disable-l1-verification flag if L1 verification is not required")
		}

		var l1Client *l1.Client
		l1Client, err = newL1Client(cfg.EthNode, cfg.Metrics, n.blockchain, n.log)
		if err != nil {
			return nil, fmt.Errorf("create L1 client: %w", err)
		}
		n.services = append(n.services, l1Client)
		rpcHandler.WithL1Client(l1Client.L1())
	}

	if semversion, err := semver.NewVersion(version); err == nil {
		ug := upgrader.NewUpgrader(semversion, githubAPIUrl, latestReleaseURL, upgraderDelay, n.log)
		n.services = append(n.services, ug)
	} else {
		log.Warnw("Failed to parse Juno version, will not warn about new releases", "version", version)
	}

	return n, nil
}

func newL1Client(ethNode string, includeMetrics bool, chain *blockchain.Blockchain, log utils.SimpleLogger) (*l1.Client, error) {
	ethNodeURL, err := url.Parse(ethNode)
	if err != nil {
		return nil, fmt.Errorf("parse Ethereum node URL: %w", err)
	}
	if ethNodeURL.Scheme != "wss" && ethNodeURL.Scheme != "ws" {
		return nil, errors.New("non-websocket Ethereum node URL (need wss://... or ws://...): " + ethNode)
	}

	network := chain.Network()

	var ethSubscriber *l1.EthSubscriber
	ethSubscriber, err = l1.NewEthSubscriber(ethNode, network.CoreContractAddress)
	if err != nil {
		return nil, fmt.Errorf("set up ethSubscriber: %w", err)
	}

	l1Client := l1.NewClient(ethSubscriber, chain, log)

	if includeMetrics {
		l1Client.WithEventListener(makeL1Metrics(chain, ethSubscriber))
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

	defer func() {
		if dbErr := n.blockchain.WriteRunningEventFilter(); dbErr != nil {
			n.log.Errorw("Error while storing running event filter", "err", dbErr)
		}
	}()

	defer func() {
		if err := n.blockchain.Stop(); err != nil {
			n.log.Errorw("Error while stopping the blockchain", "err", err)
		}
		n.log.Infow("TrieDB Journal saved")
	}()

	cfg := make(map[string]any)
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

	for _, s := range n.earlyServices {
		n.StartService(wg, ctx, cancel, s)
	}

	migrationHTTPConfig := migration.HTTPConfig{
		Enabled: n.cfg.HTTP,
		Host:    n.cfg.HTTPHost,
		Port:    n.cfg.HTTPPort,
	}

	if err := migration.MigrateIfNeeded(ctx, n.db, &n.cfg.Network, n.log, &migrationHTTPConfig); err != nil {
		if errors.Is(err, context.Canceled) {
			n.log.Infow("DB Migration cancelled")
			return
		}
		n.log.Errorw("Error while migrating the DB", "err", err)
		return
	}

	if n.cfg.Sequencer {
		// Custom networks are not supported in sequencer mode yet
		if !slices.Contains(utils.KnownNetworkNames, n.cfg.Network.Name) {
			n.log.Errorw("Custom networks are not supported in sequencer mode yet")
			return
		}
		feeTokens := utils.DefaultFeeTokenAddresses
		chainInfo := vm.ChainInfo{
			ChainID:           n.cfg.Network.L2ChainID,
			FeeTokenAddresses: feeTokens,
		}

		err := buildGenesis(
			n.cfg.SeqGenesisFile,
			n.blockchain,
			vm.New(&chainInfo, false, n.log),
			n.cfg.RPCCallMaxSteps,
			n.cfg.RPCCallMaxGas,
		)
		if err != nil {
			n.log.Errorw("Error building genesis state", "err", err)
			return
		}
	}

	for _, s := range n.services {
		n.StartService(wg, ctx, cancel, s)
	}

	<-ctx.Done()
	n.log.Infow("Shutting down Juno...")
}

func (n *Node) StartService(wg *conc.WaitGroup, ctx context.Context, cancel context.CancelFunc, s service.Service) {
	wg.Go(func() {
		// Immediately acknowledge panicing services by shutting down the node
		// Without the deffered cancel(), we would have to wait for user to hit Ctrl+C
		defer cancel()
		if err := s.Run(ctx); err != nil {
			n.log.Errorw("Service error", "name", reflect.TypeOf(s), "err", err)
		}
	})
}

func (n *Node) Config() Config {
	return *n.cfg
}
