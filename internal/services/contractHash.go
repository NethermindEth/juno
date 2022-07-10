package services

import (
	"context"
	"math/big"

	"github.com/NethermindEth/juno/internal/db"
	. "github.com/NethermindEth/juno/internal/log"
)

var ContractHashService contractHashService

type contractHashService struct {
	service
	db db.Database
}

func (s *contractHashService) Setup(database db.Database) {
	if s.Running() {
		// notest
		s.logger.Panic("trying to Setup with service running")
	}
	s.db = database
}

func (s *contractHashService) Run() error {
	if s.logger == nil {
		s.logger = Logger.Named("ContractHash Service")
	}

	if err := s.service.Run(); err != nil {
		return err
	}

	return s.setDefaults()
}

func (s *contractHashService) setDefaults() error {
	if s.db == nil {
		// notest
		env, err := db.GetMDBXEnv()
		if err != nil {
			return err
		}
		database, err := db.NewMDBXDatabase(env, "CONTRACT_HASH")
		if err != nil {
			return err
		}
		s.db = database
	}
	return nil
}

func (s *contractHashService) Close(ctx context.Context) {
	// notest
	if !s.Running() {
		return
	}
	s.service.Close(ctx)
	s.db.Close()
}

func (s *contractHashService) StoreContractHash(contractAddress string, contractHash *big.Int) {
	s.AddProcess()
	defer s.DoneProcess()

	s.logger.
		With("contractAddress", contractAddress).
		Debug("StoreContractHash")

	err := s.db.Put([]byte(contractAddress), contractHash.Bytes())
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

	rawData, err := s.db.Get([]byte(contractAddress))
	if err != nil {
		s.logger.
			With("error", err).
			Error("GetContractHash error")
		return nil
	}
	return new(big.Int).SetBytes(rawData)
}
