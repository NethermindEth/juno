package node

import (
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/starknetdata/gateway"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
)

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
	cfg          *Config
	db           db.DB
	blockchain   *blockchain.Blockchain
	synchronizer *sync.Synchronizer
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
	log.Println("Running Juno with config: ", fmt.Sprintf("%+v", *n.cfg))

	var err error
	n.db, err = db.NewDb(n.cfg.DatabasePath)
	if err != nil {
		return err
	}
	n.blockchain = blockchain.NewBlockchain(n.db)
	n.synchronizer = sync.NewSynchronizer(n.blockchain, gateway.NewGateway(n.cfg.Network))
	return n.synchronizer.Run()
}

func (n *Node) Shutdown() error {
	defer n.db.Close()
	log.Println("Shutting down Juno...")

	return n.synchronizer.Shutdown()
}
