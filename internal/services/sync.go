package services

import (
	"context"
	"math/big"

	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/db/sync"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/types"
)

// SyncService is a service to manage persistant state required to
// synchronize with the network. It must be configured with the Setup
// method; otherwise, the value will be the default global singleton.
// To stop the service, call the Close method.
var SyncService syncService

type syncService struct {
	service
	manager *sync.Manager
}

func (s *syncService) Setup(database db.DatabaseTransactional) {
	if s.Running() {
		// notest
		s.logger.Panic("trying to Setup with service running")
	}
	s.manager = sync.NewManager(database)
}

func (s *syncService) Run() error {
	if s.logger == nil {
		s.logger = log.Default.Named("Sync Service")
	}

	if err := s.service.Run(); err != nil {
		// notest
		return err
	}

	return s.setDefaults()
}

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
		s.manager = sync.NewManager(database)
	}
	return nil
}

func (s *syncService) Close(ctx context.Context) {
	// notest
	if !s.Running() {
		return
	}
	s.service.Close(ctx)
	s.manager.Close()
}

func (s *syncService) GetLatestBlockNumber() (*uint64, error) {
	return s.manager.GetLatestBlockNumber()
}

func (s *syncService) setLatestBlockNumber(blockNum uint64) error {
	return s.manager.SetLatestBlockNumber(blockNum)
}

func (s *syncService) UpdateState(update types.StateUpdate) error {
	// Save contract hashes of the new contracts
	for _, deployedContract := range update.StateDiff.DeployedContracts {
		contractHash, ok := new(big.Int).SetString(remove0x(deployedContract.Hash), 16)
		if !ok {
			// notest
			log.Default.Panic("Couldn't get contract hash")
		}
		ContractHashService.StoreContractHash(remove0x(deployedContract.Address), contractHash)
	}
	// Build contractAddress-contractHash map
	contractHashMap := make(map[string]*big.Int)
	for contractAddress := range update.StateDiff.StorageDiffs {
		formattedAddress := remove0x(contractAddress)
		contractHashMap[formattedAddress] = ContractHashService.GetContractHash(formattedAddress)
	}

	if err := s.manager.UpdateState(update, contractHashMap); err != nil {
		log.Default.Fatal(err)
	}

	if err := s.setLatestBlockNumber(update.NewBlockNumber); err != nil {
		log.Default.With("Error", err).Info("Couldn't save latest block queried")
		return err
	}

	return nil
}

// removeOx remove the initial zeros and x at the beginning of the string
func remove0x(s string) string {
	answer := ""
	found := false
	for _, char := range s {
		found = found || (char != '0' && char != 'x')
		if found {
			answer = answer + string(char)
		}
	}
	if len(answer) == 0 {
		return "0"
	}
	return answer
}
