package services

import (
	"context"
	"path/filepath"

	"github.com/NethermindEth/juno/internal/config"
	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/db/abi"
	"github.com/NethermindEth/juno/internal/log"
)

// AbiService is the service to store and put the contracts ABI. Before
// using the service, it must be configured with the Setup method;
// otherwise, the value will be the default. To stop the service, call the
// Close method.
var AbiService abiService

type abiService struct {
	service
	manager *abi.Manager
}

// Setup sets the service configuration, service must be not running.
func (s *abiService) Setup(database db.Databaser) {
	if s.Running() {
		// notest
		s.logger.Panic("trying to Setup with service running")
	}
	s.manager = abi.NewABIManager(database)
}

// Run starts the service.
func (s *abiService) Run() error {
	if s.logger == nil {
		s.logger = log.Default.Named("AbiService")
	}

	if err := s.service.Run(); err != nil {
		// notest
		return err
	}

	s.setDefaults()
	return nil
}

// setDefaults sets the default value for properties that are not set.
func (s *abiService) setDefaults() {
	if s.manager == nil {
		// notest
		database := db.NewKeyValueDb(filepath.Join(config.Runtime.DbPath, "abi"), 0)
		s.manager = abi.NewABIManager(database)
	}
}

// Close closes the service.
func (s *abiService) Close(ctx context.Context) {
	s.service.Close(ctx)
	s.manager.Close()
}

// StoreAbi stores an ABI in the database. If the key (contractAddress) already
// exists then the value is overwritten for the given ABI.
func (s *abiService) StoreAbi(contractAddress string, abi *abi.Abi) {
	s.service.AddProcess()
	defer s.service.DoneProcess()

	s.logger.
		With("contractAddress", contractAddress).
		Info("StoreAbi")

	s.manager.PutABI(contractAddress, abi)
}

// GetAbi search in the database for the ABI associated with the given contract
// address.
func (s *abiService) GetAbi(contractAddress string) *abi.Abi {
	s.service.AddProcess()
	defer s.service.DoneProcess()

	s.logger.
		With("contractAddress", contractAddress).
		Info("GetAbi")

	return s.manager.GetABI(contractAddress)
}
