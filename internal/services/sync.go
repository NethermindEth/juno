package services

import (
	"context"
	"github.com/NethermindEth/juno/internal/config"
	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/db/state"
	"github.com/NethermindEth/juno/internal/db/sync"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/feeder"
	starknetTypes "github.com/NethermindEth/juno/pkg/starknet/types"
	state2 "github.com/NethermindEth/juno/pkg/state"
	"github.com/NethermindEth/juno/pkg/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
)

// SyncService is the service that handle the synchronization of the node.
var SyncService syncService

type syncService struct {
	service
	// manager is the sync manager.
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
	// stateManager represent the manager for the state
	stateManager state2.StateManager
	// state represent the state of the trie
	state state2.State
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
	if config.Runtime.Starknet.ApiSync {
		NewApiCollector(SyncService.manager, SyncService.feeder, SyncService.chainId)
		SyncService.stateDiffCollector = APICollector
	} else {
		NewL1Collector(SyncService.manager, SyncService.feeder, SyncService.ethClient, SyncService.chainId)
		SyncService.stateDiffCollector = L1Collector
	}
	go func() {
		err = SyncService.stateDiffCollector.Run()
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

	// Get state
	for stateDiff := range s.stateDiffCollector.GetChannel() {

		if s.preValidateStateDiff(stateDiff) {
			s.logger.With("Old State Root from StateDiff", stateDiff.OldRoot,
				"Current State Root", s.state.Root().Hex(),
				"Block Number", s.latestBlockSynced+1).
				Error("Fail validation before apply StateDiff")
			continue
		}

		err := s.updateState(stateDiff)
		if err != nil || s.postValidateStateDiff(stateDiff) {
			// In case some errors exist or the new root of the trie didn't match with
			// the root we receive from the StateDiff, we have to revert the trie
			stateRoot := s.manager.GetLatestStateRoot()
			root := types.HexToFelt(stateRoot)
			s.logger.With("State Root from StateDiff", stateDiff.NewRoot,
				"State Root after StateDiff", s.state.Root().Hex(),
				"Block Number", s.latestBlockSynced+1).
				Error("Fail validation after apply StateDiff")
			s.state = state2.New(s.stateManager, &root)
			continue
		}
		s.logger.With("Block Number", stateDiff.BlockNumber,
			"Missing Blocks to fully Sync", s.stateDiffCollector.GetLatestBlockOnChain()-stateDiff.BlockNumber).
			Info("Synced block")
		s.manager.StoreLatestBlockSync(stateDiff.BlockNumber)
		s.manager.StoreLatestStateRoot(s.state.Root().Hex())
		s.latestBlockSynced = stateDiff.BlockNumber

	}
	return nil
}

func (s *syncService) postValidateStateDiff(stateDiff *starknetTypes.StateDiff) bool {
	return remove0x(s.state.Root().Hex()) != remove0x(stateDiff.NewRoot)
}

func (s *syncService) preValidateStateDiff(stateDiff *starknetTypes.StateDiff) bool {
	// The old state root that comes with the stateDiff should match with the current stateRoot
	return remove0x(s.state.Root().Hex()) != remove0x(stateDiff.OldRoot) &&
		// Should be the next block in the chain
		s.latestBlockSynced+1 == stateDiff.BlockNumber
}

func (s *syncService) updateState(stateDiff *starknetTypes.StateDiff) error {
	for _, deployedContract := range stateDiff.DeployedContracts {
		address := types.HexToFelt(deployedContract.Address)
		contractHash := types.HexToFelt(deployedContract.ContractHash)
		err := s.state.SetContractHash(&address, &contractHash)
		if err != nil {
			return err
		}
	}
	for k, v := range stateDiff.StorageDiffs {
		for _, storageSlots := range v {
			address := types.HexToFelt(k)
			slotKey := types.HexToFelt(storageSlots.Key)
			slotValue := types.HexToFelt(storageSlots.Value)
			err := s.state.SetSlot(&address, &slotKey, &slotValue)
			if err != nil {
				return err
			}
		}
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
		codeDatabase, err := db.NewMDBXDatabase(env, "CODE")
		if err != nil {
			return err
		}
		binaryDatabase, err := db.NewMDBXDatabase(env, "BINARY_DATABASE")
		if err != nil {
			return err
		}
		stateDatabase, err := db.NewMDBXDatabase(env, "STATE")
		if err != nil {
			return err
		}
		s.manager = sync.NewSyncManager(database)

		s.stateManager = state.NewStateManager(stateDatabase, binaryDatabase, codeDatabase)

		stateRoot := s.manager.GetLatestStateRoot()
		root := types.HexToFelt(stateRoot)

		s.state = state2.New(s.stateManager, &root)
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

func initStateManager() *state2.StateManager {

}
