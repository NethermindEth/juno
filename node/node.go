package node

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/clients/gateway"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/ethereum"
	"github.com/NethermindEth/juno/l1"
	"github.com/NethermindEth/juno/metrics"
	"github.com/NethermindEth/juno/migration"
	"github.com/NethermindEth/juno/node/http"
	"github.com/NethermindEth/juno/p2p"
	"github.com/NethermindEth/juno/rpc"
	"github.com/NethermindEth/juno/service"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/upgrader"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/sourcegraph/conc"
)

const (
	upgraderDelay    = 5 * time.Minute
	githubAPIUrl     = "https://api.github.com/repos/NethermindEth/juno/releases/latest"
	latestReleaseURL = "https://github.com/NethermindEth/juno/releases/latest"
)

// Config is the top-level juno configuration.
type Config struct {
	LogLevel            utils.LogLevel `mapstructure:"log-level"`
	HTTPPort            uint16         `mapstructure:"http-port"`
	DatabasePath        string         `mapstructure:"db-path"`
	Network             utils.Network  `mapstructure:"network"`
	EthNode             string         `mapstructure:"eth-node"`
	Pprof               bool           `mapstructure:"pprof"`
	Colour              bool           `mapstructure:"colour"`
	PendingPollInterval time.Duration  `mapstructure:"pending-poll-interval"`

	Metrics bool `mapstructure:"metrics"`

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
func New(cfg *Config, version string) (*Node, error) { //nolint:gocyclo
	metrics.Enabled = cfg.Metrics

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
		return nil, fmt.Errorf("create DB logger: %w", err)
	}

	database, err := pebble.New(cfg.DatabasePath, dbLog)
	if err != nil {
		return nil, fmt.Errorf("open DB: %w", err)
	}
	ua := fmt.Sprintf("Juno/%s Starknet Client", version)

	chain := blockchain.New(database, cfg.Network, log)
	client := feeder.NewClient(cfg.Network.FeederURL()).WithUserAgent(ua)
	synchronizer := sync.New(chain, adaptfeeder.New(client), log, cfg.PendingPollInterval)
	gatewayClient := gateway.NewClient(cfg.Network.GatewayURL(), log).WithUserAgent(ua)

	rpcHandler := rpc.New(chain, synchronizer, cfg.Network, gatewayClient, client, vm.New(), version, log)
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.HTTPPort))
	if err != nil {
		return nil, fmt.Errorf("listen on http port %d: %w", cfg.HTTPPort, err)
	}
	rpcSrv, err := junohttp.New(listener, cfg.Metrics, database, version, rpcHandler, cfg.Pprof, log)
	if err != nil {
		return nil, fmt.Errorf("create RPC servers: %w", err)
	}

	n := &Node{
		cfg:        cfg,
		log:        log,
		version:    version,
		db:         database,
		blockchain: chain,
		services:   []service.Service{rpcSrv, synchronizer},
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

		n.services = append(n.services, l1Client)
	}

	if cfg.P2P {
		var privKeyStr string
		privKeyStr, _ = os.LookupEnv("P2P_PRIVATE_KEY")
		var p2pService *p2p.Service
		p2pService, err = p2p.New(cfg.P2PAddr, "juno", cfg.P2PBootPeers, privKeyStr, cfg.Network, log)
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
	ethSubscriber, err := ethereum.DialWithContext(context.Background(), ethNode, chain.Network(), log)
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

	if err := migration.MigrateIfNeeded(n.db, n.cfg.Network, n.log); err != nil {
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
