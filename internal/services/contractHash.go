package services

import (
	"context"
	"github.com/NethermindEth/juno/internal/db/contractHash"
	"math/big"

	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/log"
)

var ContractHashService contractHashService

type contractHashService struct {
	service
	manager *contractHash.Manager
}

func (s *contractHashService) Setup(database db.Database) {
	if s.Running() {
		// notest
		s.logger.Panic("trying to Setup with service running")
	}
	s.manager = contractHash.NewManager(database)
}

func (s *contractHashService) Run() error {
	if s.logger == nil {
		s.logger = log.Default.Named("ContractHash Service")
	}

	if err := s.service.Run(); err != nil {
		return err
	}

	return s.setDefaults()
}

func (s *contractHashService) setDefaults() error {
	if s.manager == nil {
		// notest
		database, err := db.NewMDBXDatabase("CONTRACT_HASH")
		if err != nil {
			return err
		}
		s.manager = contractHash.NewManager(database)
	}
	return nil
}

func (s *contractHashService) Close(ctx context.Context) {
	// notest
	if !s.Running() {
		return
	}
	s.service.Close(ctx)
	s.manager.Close()
}

func (s *contractHashService) StoreContractHash(contractAddress string, contractHash *big.Int) {
	s.AddProcess()
	defer s.DoneProcess()

	s.logger.
		With("contractAddress", contractAddress).
		Debug("StoreContractHash")

	err := s.manager.StoreContractHash(contractAddress, contractHash)
	if err != nil {
		// notest
		s.logger.
			With("error", err).
			Error("StoreContractHash error")
	}
}

func (s *contractHashService) GetContractHash(contractAddress string) *big.Int {
	// notest
	s.AddProcess()
	defer s.DoneProcess()

	s.logger.
		With("contractAddress", contractAddress).
		Debug("GetContractHash")

	return s.manager.GetContractHash(contractAddress)
}
