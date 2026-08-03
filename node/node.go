package node

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/clients/gateway"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/remote"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/node/upgrader"
	"github.com/NethermindEth/juno/p2p"
	"github.com/NethermindEth/juno/plugin"
	"github.com/NethermindEth/juno/pruner"
	"github.com/NethermindEth/juno/rpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/starknet/compiler"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/NethermindEth/juno/vm"
	"github.com/sourcegraph/conc"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/yaml.v3"
)

const (
	PruneModeFlag   = "prune-mode"
	PruneMinAgeFlag = "prune-min-age"
)

type Node struct {
	cfg        *Config
	db         db.KeyValueStore
	blockchain *blockchain.Blockchain
	// Services that needs to start before than other services and before migration.
	earlyServices []service.Service
	services      []service.Service
	logger        log.Logger

	version string
}

// New sets the config and logger to the StarknetNode.
// Any errors while parsing the config on creating logger will be returned.
// Todo: (immediate follow-up PR) tidy this function up.
//
//nolint:gocyclo,funlen // TODO: refactor this function to reduce complexity
func New(
	ctx context.Context,
	cfg *Config,
	version string,
	logLevel *log.Level,
) (*Node, error) {
	if cfg.Sequencer {
		return nil, errors.New("sequencer configuration is no longer supported")
	}

	// History pruning needs an L1-finalised cutoff to know which blocks are
	// safe to drop. If no L1 client is given, new cutoffs cannot be set.
	if cfg.Prune && cfg.DisableL1Verification {
		return nil, errors.New("--prune-mode requires L1 verification; " +
			"remove --disable-l1-verification or disable --prune-mode")
	}

	logger, err := log.NewZapLogger(
		logLevel,
		log.WithColour(cfg.Colour),
		log.WithJSON(cfg.LogJSON),
	)
	if err != nil {
		return nil, err
	}

	isRemoteDB := cfg.RemoteDB != ""
	var database db.KeyValueStore
	if isRemoteDB {
		database, err = remote.New(
			ctx,
			cfg.RemoteDB,
			logger,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			return nil, fmt.Errorf("opening remote DB: %w", err)
		}
	} else {
		database, err = initializeLocalDB(cfg)
		if err != nil {
			return nil, fmt.Errorf("opening local DB: %w", err)
		}
	}

	chainOpts := make([]blockchain.Option, 0, 3)
	chainOpts = append(chainOpts, blockchain.WithNewState(cfg.NewState))
	if cfg.Metrics {
		chainOpts = append(chainOpts, blockchain.WithListener(makeBlockchainMetrics()))
	}
	if cfg.Prune {
		chainOpts = append(
			chainOpts,
			blockchain.WithRunningEventFilterInitializer(pruner.InitializeRunningEventFilter),
		)
	}
	chain := blockchain.New(database, &cfg.Network, chainOpts...)

	// Verify that cfg.Network is compatible with the database.
	head, err := chain.Head()
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, fmt.Errorf("getting head block from database: %w", err)
	}
	if head != nil {
		stateUpdate, err := chain.StateUpdateByNumber(head.Number)
		if err != nil {
			return nil, err
		}

		// We assume that there is at least one transaction in the block
		// or that it is a pre-0.7 block.
		trieBackend := core.DeprecatedTrieBackend
		if cfg.NewState {
			trieBackend = core.TrieBackend
		}
		_, err = core.VerifyBlockHash(head, &cfg.Network, stateUpdate.StateDiff, trieBackend)
		if err != nil {
			return nil, errors.New(
				"unable to verify latest block hash; " +
					"are the database and --network option compatible?")
		}
	}

	if cfg.VersionedConstantsFile != "" {
		err = vm.SetVersionedConstants(cfg.VersionedConstantsFile)
		if err != nil {
			return nil, fmt.Errorf("setting custom versioned constants: %w", err)
		}
	}

	services := make([]service.Service, 0)

	var junoPlugin plugin.JunoPlugin
	if cfg.PluginPath != "" {
		junoPlugin, err = plugin.Load(cfg.PluginPath)
		if err != nil {
			return nil, err
		}
		services = append(services, plugin.NewService(junoPlugin))
	}

	maxConcurrentComp, maxQueuedComp := calculateCompilerConcurrencyBudget(cfg, logger)
	compiler := compiler.New(
		&compiler.Config{
			MaxMemory:  uint64(cfg.MaxCompilationMemory) * 1024 * 1024,
			MaxCPUTime: uint64(cfg.MaxCompilationCPUTime),
		},
		"",
		logger,
	)
	throttledCompiler := NewThrottledCompiler(compiler, uint(maxConcurrentComp), maxQueuedComp)

	userAgentID := fmt.Sprintf("Juno/%s Starknet Client", version)
	timeouts, fixed, err := feeder.ParseTimeouts(cfg.GatewayTimeouts)
	if err != nil {
		return nil, fmt.Errorf("invalid gateway timeouts: %w", err)
	}
	if cfg.Network.FeederURL == nil {
		return nil, fmt.Errorf("network %q has no feeder URL configured", cfg.Network.Name)
	}
	feederClient := feeder.NewClient(
		cfg.Network.FeederURL,
		feeder.WithUserAgent(userAgentID),
		feeder.WithLogger(logger),
		feeder.WithTimeouts(timeouts, fixed),
		feeder.WithAPIKey(cfg.GatewayAPIKey),
		feeder.WithListener(makeFeederMetrics(cfg.Metrics)),
	)

	// Handle fee tokens for custom networks
	feeTokens := networks.DefaultFeeTokenAddresses
	if !slices.Contains(networks.KnownNetworkNames, cfg.Network.Name) {
		// For custom networks, fetch fee tokens from the gateway
		feeTokens, err = feederClient.FeeTokenAddresses(ctx)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to fetch fee token addresses for custom network: %w", err,
			)
		}
	}

	chainInfo := vm.ChainInfo{
		ChainID:           cfg.Network.L2ChainID,
		FeeTokenAddresses: feeTokens,
	}
	nodeVM := vm.New(&chainInfo, false, logger)
	throttledVM := NewThrottledVM(nodeVM, cfg.MaxVMs, uint64(cfg.MaxVMQueue))

	feederGatewayDataSource := sync.NewFeederGatewayDataSource(chain, adaptfeeder.New(feederClient))
	synchronizer := sync.New(
		chain,
		feederGatewayDataSource,
		logger,
		cfg.PreConfirmedPollInterval,
		isRemoteDB,
		database,
	)
	synchronizer.WithPlugin(junoPlugin)

	if cfg.Network.GatewayURL == nil {
		return nil, fmt.Errorf("network %q has no gateway URL configured", cfg.Network.Name)
	}
	gatewayClient := gateway.NewClient(cfg.Network.GatewayURL, logger).
		WithUserAgent(userAgentID).
		WithAPIKey(cfg.GatewayAPIKey)

	var p2pService *p2p.Service
	if cfg.P2P {
		if cfg.Network == networks.Mainnet {
			return nil, fmt.Errorf("P2P cannot be used on %v network", networks.Mainnet)
		}
		logger.Warn("P2P features enabled. Please note P2P is in experimental stage")

		if !cfg.P2PFeederNode {
			// Do not start the feeder synchronisation
			synchronizer = nil
		}
		p2pService, err = p2p.New(
			cfg.P2PAddr,
			cfg.P2PPublicAddr,
			version,
			cfg.P2PPeers,
			cfg.P2PPrivateKey,
			cfg.P2PFeederNode,
			chain,
			&cfg.Network,
			logger,
			database,
			throttledCompiler,
		)
		if err != nil {
			return nil, fmt.Errorf("set up p2p service: %w", err)
		}

		services = append(services, p2pService)
	}

	var syncReader sync.Reader = &sync.NoopSynchronizer{}
	if synchronizer != nil {
		syncReader = synchronizer
	}

	submittedTransactionsCache := rpccore.NewTransactionCache(
		cfg.SubmittedTransactionsCacheEntryTTL,
		cfg.SubmittedTransactionsCacheSize,
	)
	services = append(services, submittedTransactionsCache)

	if synchronizer != nil {
		services = append(services, synchronizer)
		if cfg.Prune {
			p := pruner.New(
				database,
				cfg.RetainedBlocks,
				synchronizer.SubscribeNewHeads().Subscription,
				chain.SubscribeL1Head().Subscription,
				logger,
				pruner.WithListener(makePrunerMetrics(cfg.Metrics)),
				pruner.WithMinAge(cfg.PruneMinAge),
			)
			services = append(services, p)
		}
	}

	rpcHandler := rpc.New(chain, syncReader, throttledVM, version, logger, &cfg.Network).
		WithCompiler(throttledCompiler).
		WithGateway(gatewayClient).
		WithFeeder(feederClient).
		WithSubmittedTransactionsCache(submittedTransactionsCache).
		WithFilterLimit(cfg.RPCMaxBlockScan).
		WithCallMaxSteps(cfg.RPCCallMaxSteps).
		WithCallMaxGas(cfg.RPCCallMaxGas)

	if !cfg.DisableReceivedTxnStream {
		receivedTxFeed := feed.New[core.Transaction]()
		rpcHandler = rpcHandler.WithReceivedTransactionFeed(receivedTxFeed)
	}
	services = append(services, rpcHandler)

	rpcServers, err := makeRPCServers(cfg, rpcHandler, logger)
	if err != nil {
		return nil, fmt.Errorf("building rpc servers: %w", err)
	}

	if cfg.HTTP {
		readinessHandlers := NewReadinessHandlers(chain, syncReader, cfg.ReadinessBlockTolerance)
		httpHandlers := map[string]http.HandlerFunc{
			"/live":       readinessHandlers.HandleLive,
			"/ready":      readinessHandlers.HandleReadySync,
			"/ready/sync": readinessHandlers.HandleReadySync,
		}
		services = append(
			services,
			makeRPCOverHTTP(
				cfg.HTTPHost,
				cfg.HTTPPort,
				rpcServers,
				httpHandlers,
				logger,
				cfg.Metrics,
				cfg.RPCCorsEnable,
				cfg.RPCRequestTimeout,
				cfg.RPCMaxConcurrentRequests,
				cfg.RPCMaxRequestQueue,
			),
		)
	}

	if cfg.Websocket {
		services = append(
			services,
			makeRPCOverWebsocket(
				cfg.WebsocketHost,
				cfg.WebsocketPort,
				rpcServers,
				logger,
				cfg.Metrics,
				cfg.RPCCorsEnable,
				cfg.RPCRequestTimeout,
			),
		)
	}

	earlyServices := make([]service.Service, 0)

	if cfg.HTTPUpdatePort != 0 {
		logger.Info(
			"Log level and feeder gateway timeouts can be changed via HTTP PUT request to " +
				cfg.HTTPUpdateHost + ":" + fmt.Sprintf("%d", cfg.HTTPUpdatePort) +
				"/log/level and /feeder/timeouts",
		)
		earlyServices = append(
			earlyServices,
			makeHTTPUpdateService(cfg.HTTPUpdateHost, cfg.HTTPUpdatePort, logLevel, feederClient),
		)
	}
	if cfg.Metrics {
		makeJeMallocMetrics()
		makeVMThrottlerMetrics(throttledVM)
		makeCompilerThrottlerMetrics(throttledCompiler)
		makePebbleMetrics(database)
		makeJunoMetrics(version)
		database.WithListener(makeDBMetrics())
		gatewayClient.WithListener(makeGatewayMetrics())
		if synchronizer != nil {
			synchronizer.WithListener(makeSyncMetrics(synchronizer, chain))
		} else if p2pService != nil {
			// regular p2p node
			p2pService.WithListener(makeSyncMetrics(&sync.NoopSynchronizer{}, chain))
		}

		earlyServices = append(earlyServices, makeMetrics(cfg.MetricsHost, cfg.MetricsPort))
	}
	if cfg.GRPC {
		services = append(services, makeGRPC(cfg.GRPCHost, cfg.GRPCPort, database, version))
	}
	if cfg.Pprof {
		services = append(services, makePPROF(cfg.PprofHost, cfg.PprofPort))
	}

	node := &Node{
		cfg:           cfg,
		logger:        logger,
		version:       version,
		db:            database,
		blockchain:    chain,
		services:      services,
		earlyServices: earlyServices,
	}

	if !node.cfg.DisableL1Verification {
		// Due to mutually exclusive flag we can do the following.
		if node.cfg.EthNode == "" {
			return nil, fmt.Errorf("ethereum node address not set; " +
				"Use --disable-l1-verification flag if L1 verification is not required",
			)
		}

		l1Client, provider, err := newL1Client(
			ctx, cfg.EthNode, cfg.Metrics, node.blockchain, node.logger,
		)
		if err != nil {
			return nil, fmt.Errorf("initializing L1 client: %w", err)
		}

		node.services = append(node.services, l1Client)
		rpcHandler.WithL1Client(provider)
	}

	if semversion, err := semver.NewVersion(version); err == nil {
		const upgraderDelay = 5 * time.Minute
		const githubAPIUrl = "https://api.github.com/repos/NethermindEth/juno/releases/latest"
		const latestReleaseURL = "https://github.com/NethermindEth/juno/releases/latest"
		ug := upgrader.NewUpgrader(
			semversion, githubAPIUrl, latestReleaseURL, upgraderDelay, node.logger,
		)
		node.services = append(node.services, ug)
	} else {
		logger.Warn(
			"Failed to parse Juno version, will not warn about new releases",
			zap.String("version", version),
		)
	}

	return node, nil
}

// Run starts Juno node by opening the DB, initialising services.
// All the services blocking and any errors returned by service run function is logged.
// Run will wait for all services to return before exiting.
func (n *Node) Run(ctx context.Context) {
	defer func() {
		if closeErr := n.db.Close(); closeErr != nil {
			n.logger.Error("Error while closing the DB", zap.Error(closeErr))
		}
	}()

	defer func() {
		if dbErr := n.blockchain.WriteRunningEventFilter(); dbErr != nil {
			n.logger.Error("Error while storing running event filter", zap.Error(dbErr))
		}
	}()

	yamlConfig, err := yaml.Marshal(n.cfg)
	if err != nil {
		n.logger.Error("Error while marshalling config", zap.Error(err))
		return
	}
	n.logger.Debug(fmt.Sprintf("Running Juno with config:\n%s", string(yamlConfig)))

	wg := conc.NewWaitGroup()
	defer wg.Wait()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for _, s := range n.earlyServices {
		n.StartService(wg, ctx, cancel, s)
	}

	err = migrateIfNeeded(ctx, n.db, n.cfg, n.blockchain, n.logger)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			n.logger.Info("DB Migration cancelled")
			return
		}
		n.logger.Error("Error while running migrations", zap.Error(err))
		return
	}

	for _, s := range n.services {
		n.StartService(wg, ctx, cancel, s)
	}

	<-ctx.Done()
	n.logger.Info("Shutting down Juno...")
}

func (n *Node) StartService(
	wg *conc.WaitGroup, ctx context.Context, cancel context.CancelFunc, s service.Service,
) {
	wg.Go(func() {
		// Immediately acknowledge panicing services by shutting down the node
		// Without the deffered cancel(), we would have to wait for user to hit Ctrl+C
		defer cancel()
		if err := s.Run(ctx); err != nil {
			n.logger.Error(
				"Service error",
				zap.String("name", reflect.TypeOf(s).String()),
				zap.Error(err),
			)
		}
	})
}

func (n *Node) Config() Config {
	return *n.cfg
}
