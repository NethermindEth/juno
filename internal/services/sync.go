package services

import (
	"context"
	"github.com/NethermindEth/juno/internal/config"
	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/db/sync"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
)

// SyncService is the service that handle the synchronization of the node.
var SyncService syncService

type syncService struct {
	service
	manager *sync.Manager
	// feeder is the client that will be used to fetch the data that comes from the Feeder Gateway.
	feeder *feeder.Client
	// ethClient is the client that will be used to fetch the data that comes from the Ethereum Node.
	ethClient *ethclient.Client
	// chainId represent the chain id of the node.
	chainId int
	// latestBlockSynced is the last block that was synced.
	latestBlockSynced int64
	// stateDIffCollector
	stateDiffCollector StateDiffCollector
}

func SetupSync(feederClient *feeder.Client, ethereumClient *ethclient.Client) {
	err := SyncService.setDefaults()
	if err != nil {
		return
	}
	SyncService.ethClient = ethereumClient
	SyncService.feeder = feederClient
	SyncService.setChainId()
	SyncService.logger = log.Default.Named("Sync Service")
	NewApiCollector(SyncService.manager, SyncService.feeder, SyncService.chainId)
	SyncService.stateDiffCollector = APICollector
	go func() {

		err = APICollector.Run()
		if err != nil {
			panic("API should initialize")
			return
		}
	}()
}

// Setup sets the service configuration, service must be not running.
func (s *syncService) Setup(database db.Database) {
	if s.Running() {
		// notest
		s.logger.Panic("trying to Setup with service running")
	}
	s.manager = sync.NewSyncManager(database)
}

// Run starts the service.
func (s *syncService) Run() error {
	if s.logger == nil {
		s.logger = log.Default.Named("SyncService")
	}

	if err := s.service.Run(); err != nil {
		// notest
		return err
	}
	go func() {
		_ = s.stateDiffCollector.Run()
	}()

	for stateDiff := range s.stateDiffCollector.GetChannel() {
		// TODO: add state diff to black box
		s.logger.With("Block Number", stateDiff.BlockNumber).Info("Synced block")
		s.latestBlockSynced = stateDiff.BlockNumber

	}
	return nil
}

func (s *syncService) GetLatestBlockOnChain() int64 {
	return s.stateDiffCollector.GetLatestBlockOnChain()
}

// setDefaults sets the default value for properties that are not set.
func (s *syncService) setDefaults() error {
	if s.manager == nil {
		// notest
		env, err := db.GetMDBXEnv()
		if err != nil {
			return err
		}
		database, err := db.NewMDBXDatabase(env, "SYNC")
		if err != nil {
			return err
		}
		s.manager = sync.NewSyncManager(database)
	}
	return nil
}

// Close closes the service.
func (s *syncService) Close(ctx context.Context) {
	s.service.Close(ctx)
	s.stateDiffCollector.Close(ctx)
	s.manager.Close()
}

// GetChainId returns the chain id of the node.
func (s *syncService) GetChainId() int {
	return s.chainId
}

// setChainId sets the chain id of the node.
func (s *syncService) setChainId() {

	var chainID *big.Int
	if s.ethClient == nil {
		// notest
		if config.Runtime.Starknet.Network == "mainnet" {
			chainID = new(big.Int).SetInt64(1)
		} else {
			chainID = new(big.Int).SetInt64(0)
		}
	} else {
		var err error
		chainID, err = s.ethClient.ChainID(context.Background())
		if err != nil {
			// notest
			log.Default.Panic("Unable to retrieve chain ID from Ethereum Node")
		}
	}
	s.chainId = int(chainID.Int64())
}
