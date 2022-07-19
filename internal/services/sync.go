package services

import (
	"context"
	"math/big"
	"strconv"
	"time"

	"github.com/NethermindEth/juno/pkg/felt"

	"github.com/NethermindEth/juno/internal/config"
	"github.com/NethermindEth/juno/internal/db"
	dbState "github.com/NethermindEth/juno/internal/db/state"
	syncDB "github.com/NethermindEth/juno/internal/db/sync"
	. "github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/NethermindEth/juno/pkg/state"
	"github.com/NethermindEth/juno/pkg/types"
)

// SyncService is the service that handle the synchronization of the node.
var SyncService syncService

type syncService struct {
	service
	// manager is the sync manager.
	manager *syncDB.Manager
	// feeder is the client that will be used to fetch the data that comes from the Feeder Gateway.
	feeder *feeder.Client
	// chainId represent the chain id of the node.
	chainId int
	// latestBlockSynced is the last block that was synced.
	latestBlockSynced int64
	// stateDIffCollector
	stateDiffCollector StateDiffCollector
	// stateManager represent the manager for the state
	stateManager state.StateManager
	// state represent the state of the trie
	state state.State
	// synchronizer is the synchronizer that will be used to sync all the information around the blocks
	synchronizer *Synchronizer
}

func SetupSync(feederClient *feeder.Client, l1client L1Client) {
	err := SyncService.setDefaults()
	if err != nil {
		return
	}
	SyncService.feeder = feederClient
	SyncService.setChainId(l1client)
	SyncService.logger = Logger.Named("Sync Service")
	if config.Runtime.Starknet.ApiSync {
		NewApiCollector(SyncService.manager, SyncService.feeder, SyncService.chainId)
		SyncService.stateDiffCollector = APICollector
	} else {
		NewL1Collector(SyncService.manager, SyncService.feeder, l1client, SyncService.chainId)
		SyncService.stateDiffCollector = L1Collector
	}
	// SyncService.synchronizer = NewSynchronizer(SyncService.manager, SyncService.stateManager,
	//	SyncService.feeder, SyncService.stateDiffCollector)
	go func() {
		err = SyncService.stateDiffCollector.Run()
		if err != nil {
			panic("API should initialize")
		}
	}()
}

// Run starts the service.
func (s *syncService) Run() error {
	if s.logger == nil {
		s.logger = Logger.Named("SyncService")
	}

	if err := s.service.Run(); err != nil {
		// notest
		return err
	}

	// run synchronizer of all the info that comes from the block.
	// go s.synchronizer.Run()

	// Get state
	for stateDiff := range s.stateDiffCollector.GetChannel() {
		start := time.Now()

		err := s.updateState(stateDiff)
		if err != nil || s.state.Root().Cmp(stateDiff.NewRoot) != 0 {
			// In case some errors exist or the new root of the trie didn't match with
			// the root we receive from the StateDiff, we have to revert the trie
			stateRoot := s.manager.GetLatestStateRoot()
			root := new(felt.Felt).SetHex(stateRoot)
			s.logger.With("State Root from StateDiff", stateDiff.NewRoot,
				"State Root after StateDiff", s.state.Root().Hex(),
				"Block Number", s.latestBlockSynced).
				Error("Fail validation after apply StateDiff")
			s.state = state.New(s.stateManager, root)
			continue
		}
		s.logger.With("Block Number", stateDiff.BlockNumber,
			"Missing Blocks to fully Sync", s.stateDiffCollector.GetLatestBlockOnChain()-stateDiff.BlockNumber,
			"Timer", time.Since(start)).
			Info("Synced block")
		s.manager.StoreLatestBlockSync(stateDiff.BlockNumber)
		s.manager.StoreLatestStateRoot(s.state.Root().Hex())
		s.latestBlockSynced = stateDiff.BlockNumber

	}
	return nil
}

func (s *syncService) updateState(stateDiff *types.StateDiff) error {
	for _, deployedContract := range stateDiff.DeployedContracts {
		err := s.SetCode(stateDiff, deployedContract)
		if err != nil {
			return err
		}
	}

	for contractAddress, memoryCells := range stateDiff.StorageDiff {
		for _, cell := range memoryCells {
			err := s.state.SetSlot(contractAddress, cell.Address, cell.Value)
			if err != nil {
				return err
			}
		}
	}
	s.logger.With("Block Number", stateDiff.BlockNumber).Debug("State updated")
	return nil
}

func (s *syncService) SetCode(stateDiff *types.StateDiff, deployedContract types.DeployedContract) error {
	// Get Full Contract
	contractFromApi, err := s.feeder.GetFullContractRaw(deployedContract.Address.Hex(), "", strconv.FormatInt(stateDiff.BlockNumber, 10))
	if err != nil {
		s.logger.With("Block Number", stateDiff.BlockNumber,
			"Contract Address", deployedContract.Address.Hex()).
			Error("Error getting full contract")
		return err
	}

	contract := new(types.Contract)
	err = contract.UnmarshalRaw(contractFromApi)
	if err != nil {
		s.logger.With("Block Number", stateDiff.BlockNumber,
			"Contract Address", deployedContract.Address.Hex()).
			Error("Error unmarshalling contract")
		return err
	}
	err = s.state.SetContract(deployedContract.Address, deployedContract.Hash, contract)
	if err != nil {
		s.logger.With("Block Number", stateDiff.BlockNumber,
			"Contract Address", deployedContract.Address).
			Error("Error setting code")
		return err
	}
	s.logger.With("Block Number", stateDiff.BlockNumber).Debug("State updated for Contract")
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
		contractDef, err := db.NewMDBXDatabase(env, "CONTRACT_DEF")
		if err != nil {
			return err
		}
		stateDatabase, err := db.NewMDBXDatabase(env, "STATE")
		if err != nil {
			return err
		}
		s.manager = syncDB.NewSyncManager(database)

		s.stateManager = dbState.NewStateManager(stateDatabase, contractDef)

		s.setStateToLatestRoot()
	}
	return nil
}

func (s *syncService) setStateToLatestRoot() {
	stateRoot := s.manager.GetLatestStateRoot()
	root := new(felt.Felt).SetHex(stateRoot)
	s.state = state.New(s.stateManager, root)
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
func (s *syncService) setChainId(l1client L1Client) {
	var chainID *big.Int
	if l1client == nil {
		// notest
		if config.Runtime.Starknet.Network == "mainnet" {
			chainID = new(big.Int).SetInt64(1)
		} else {
			chainID = new(big.Int).SetInt64(0)
		}
	} else {
		var err error
		chainID, err = l1client.ChainID(context.Background())
		if err != nil {
			// notest
			Logger.Panic("Unable to retrieve chain ID from Ethereum Node")
		}
	}
	s.chainId = int(chainID.Int64())
}
