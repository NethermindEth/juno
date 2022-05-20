package services

import (
	"context"
	"github.com/NethermindEth/juno/internal/config"
	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/db/abi"
	"github.com/NethermindEth/juno/internal/log"
	"math/big"
)

// AbiService is the service to store and put the contracts ABI.
var AbiService abiService

type abiService struct {
	service
	manager *abi.Manager
}

// Setup sets the service configuration, service must be not running.
func (s *abiService) Setup(database db.Databaser) {
	if s.service.Running() {
		// notest
		s.logger.Panic("try to Setup with serv")
	}
	s.manager = abi.NewABIManager(database)
}

// Run starts the service.
func (s *abiService) Run() error {
	s.logger = log.Default.Named("AbiService")
	s.setDefaults()
	if err := s.service.Run(); err != nil {
		// notest
		return nil
	}
	return nil
}

// setDefaults sets the default value for properties that are not set.
func (s *abiService) setDefaults() {
	if s.manager == nil {
		// notest
		database := db.NewKeyValueDb(config.Dir+"/abi", 0)
		s.manager = abi.NewABIManager(database)
	}
}

// Close closes the service.
func (s *abiService) Close(ctx context.Context) {
	s.manager.Close()
	s.service.Close(ctx)
}

// StoreAbi stores an ABI in the database. If the key (contractAddress) already
// exists then the value is overwritten for the given ABI.
func (s *abiService) StoreAbi(contractAddress big.Int, abi *abi.Abi) {
	s.service.AddProcess()
	defer s.service.DoneProcess()

	s.manager.PutABI(contractAddress, abi)
}

// GetAbi search in the database for the ABI associated with the given contract
// address.
func (s *abiService) GetAbi(contractAddress big.Int) *abi.Abi {
	s.service.AddProcess()
	defer s.service.DoneProcess()

	return s.manager.GetABI(contractAddress)
}
