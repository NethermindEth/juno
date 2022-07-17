package services

import (
	"context"
	"github.com/NethermindEth/juno/pkg/felt"
	"math/big"
	"strconv"
	"time"

	"github.com/NethermindEth/juno/internal/config"
	"github.com/NethermindEth/juno/internal/db"
	dbState "github.com/NethermindEth/juno/internal/db/state"
	syncDB "github.com/NethermindEth/juno/internal/db/sync"
	. "github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/NethermindEth/juno/pkg/state"
	"github.com/NethermindEth/juno/pkg/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// SyncService is the service that handle the synchronization of the node.
var SyncService syncService

type syncService struct {
	service
	// manager is the sync manager.
	manager *syncDB.Manager
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
	stateManager state.StateManager
	// state represent the state of the trie
	state state.State
	// synchronizer is the synchronizer that will be used to sync all the information around the blocks
	synchronizer *Synchronizer
}

func SetupSync(feederClient *feeder.Client, ethereumClient *ethclient.Client) {
	err := SyncService.setDefaults()
	if err != nil {
		return
	}
	SyncService.ethClient = ethereumClient
	SyncService.feeder = feederClient
	SyncService.setChainId()
	SyncService.logger = Logger.Named("Sync Service")
	if config.Runtime.Starknet.ApiSync {
		NewApiCollector(SyncService.manager, SyncService.feeder, SyncService.chainId)
		SyncService.stateDiffCollector = APICollector
	} else {
		NewL1Collector(SyncService.manager, SyncService.feeder, SyncService.ethClient, SyncService.chainId)
		SyncService.stateDiffCollector = L1Collector
	}
	// SyncService.synchronizer = NewSynchronizer(SyncService.manager, SyncService.stateManager,
	//	SyncService.feeder, SyncService.stateDiffCollector)
	go func() {
		err = SyncService.stateDiffCollector.Run()
		if err != nil {
			panic("API should initialize")
			return
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

func (s *syncService) postValidateStateDiff(stateDiff *types.StateDiff) bool {
	return remove0x(s.state.Root().Hex()) != remove0x(stateDiff.NewRoot)
}

func (s *syncService) preValidateStateDiff(stateDiff *types.StateDiff) bool {
	// The old state root that comes with the stateDiff should match with the current stateRoot
	return remove0x(s.state.Root().Hex()) != remove0x(stateDiff.OldRoot) &&
		// Should be the next block in the chain
		s.latestBlockSynced+1 == stateDiff.BlockNumber
}

func (s *syncService) updateState(stateDiff *types.StateDiff) error {

	//var wg sync.WaitGroup

	for _, deployedContract := range stateDiff.DeployedContracts {
		//wg.Add(1)

		//go func(deployedContract *types.DeployedContract) {
		//defer wg.Done()
		address := new(felt.Felt).SetHex(deployedContract.Address)
		contractHash := new(felt.Felt).SetHex(deployedContract.ContractHash)

		// Get Full Contract
		contractFromApi, err := s.feeder.GetFullContractRaw(deployedContract.Address, "",
			strconv.FormatInt(stateDiff.BlockNumber, 10))
		if err != nil {
			s.logger.With("Block Number", stateDiff.BlockNumber,
				"Contract Address", deployedContract.Address).
				Error("Error getting full contract")
			return err
		}

		//bytes, err := json.Marshal(contractFromApi)
		//if err != nil {
		//	s.logger.With("Block Number", stateDiff.BlockNumber,
		//		"Contract Address", deployedContract.Address).
		//		Error("Error updating state")
		//	return
		//}

		contract := new(types.Contract)
		err = contract.UnmarshalRaw(contractFromApi)
		if err != nil {
			s.logger.With("Block Number", stateDiff.BlockNumber,
				"Contract Address", deployedContract.Address).
				Error("Error unmarshalling contract")
			return err
		}
		err = s.state.SetCode(address, contractHash, contract)
		if err != nil {
			s.logger.With("Block Number", stateDiff.BlockNumber,
				"Contract Address", deployedContract.Address).
				Error("Error setting code")
			return err
		}
		s.logger.With("Block Number", stateDiff.BlockNumber).Debug("State updated for Contract")
		//}(&deployedContract)
	}
	//wg.Wait()
	for k, v := range stateDiff.StorageDiffs {
		for _, storageSlots := range v {
			address := new(felt.Felt).SetHex(k)
			slotKey := new(felt.Felt).SetHex(storageSlots.Key)
			slotValue := new(felt.Felt).SetHex(storageSlots.Value)
			err := s.state.SetSlot(address, slotKey, slotValue)
			if err != nil {
				return err
			}
		}
	}
	s.logger.With("Block Number", stateDiff.BlockNumber).Debug("State updated")
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
			Logger.Panic("Unable to retrieve chain ID from Ethereum Node")
		}
	}
	s.chainId = int(chainID.Int64())
}
