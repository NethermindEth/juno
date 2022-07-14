package services

import (
	"context"

	"github.com/NethermindEth/juno/internal/db"
	. "github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/felt"
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

func (s *contractHashService) StoreContractHash(contractAddress string, contractHash *felt.Felt) error {
	s.AddProcess()
	defer s.DoneProcess()
	return s.db.Put([]byte(contractAddress), contractHash.ByteSlice())
}

func (s *contractHashService) GetContractHash(contractAddress string) (*felt.Felt, error) {
	// notest
	s.AddProcess()
	defer s.DoneProcess()
	rawData, err := s.db.Get([]byte(contractAddress))
	return new(felt.Felt).SetBytes(rawData), err
}
