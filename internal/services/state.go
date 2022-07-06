package services

import (
	"context"

	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/db/state"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/ethereum/go-ethereum/common"
)

var StateService stateService

type stateService struct {
	service
	manager *state.Manager
}

func (s *stateService) Setup(codeDatabase db.Database, storageDatabase *db.BlockSpecificDatabase) {
	if s.Running() {
		// notest
		s.logger.Panic("service is already running")
	}
	s.manager = state.NewStateManager(codeDatabase, storageDatabase)
}

func (s *stateService) Run() error {
	if s.logger == nil {
		s.logger = log.Default.Named("StateService")
	}

	if err := s.service.Run(); err != nil {
		// notest
		return err
	}

	return s.setDefaults()
}

func (s *stateService) setDefaults() error {
	if s.manager == nil {
		// notest
		codeDb, err := db.NewMDBXDatabase("CODE")
		if err != nil {
			return err
		}
		storageDb, err := db.NewMDBXDatabase("STORAGE")
		if err != nil {
			return err
		}
		storageDatabase := db.NewBlockSpecificDatabase(storageDb)
		s.manager = state.NewStateManager(codeDb, storageDatabase)
	}
	return nil
}

func (s *stateService) Close(ctx context.Context) {
	// notest
	if !s.Running() {
		return
	}
	s.service.Close(ctx)
	s.manager.Close()
}

func (s *stateService) StoreCode(contractAddress []byte, code *state.Code) {
	s.AddProcess()
	defer s.DoneProcess()

	s.logger.
		With("contractAddress", common.Bytes2Hex(contractAddress)).
		Debug("StoreCode")

	s.manager.PutCode(contractAddress, code)
}

func (s *stateService) GetCode(contractAddress []byte) *state.Code {
	s.AddProcess()
	defer s.DoneProcess()

	s.logger.
		With("contractAddress", contractAddress).
		Debug("GetCode")

	return s.manager.GetCode(contractAddress)
}

func (s *stateService) StoreStorage(contractAddress string, blockNumber uint64, storage *state.Storage) {
	s.AddProcess()
	defer s.DoneProcess()

	s.logger.
		With("contractAddress", contractAddress, "blockNumber", blockNumber).
		Debug("StoreStorage")

	s.manager.PutStorage(contractAddress, blockNumber, storage)
}

func (s *stateService) GetStorage(contractAddress string, blockNumber uint64) *state.Storage {
	s.AddProcess()
	defer s.DoneProcess()

	s.logger.
		With("contractAddress", contractAddress, "blockNumber", blockNumber).
		Debug("GetStorage")

	return s.manager.GetStorage(contractAddress, blockNumber)
}

func (s *stateService) UpdateStorage(contractAddress string, blockNumber uint64, storage *state.Storage) {
	s.AddProcess()
	defer s.DoneProcess()

	s.logger.
		With("contractAddress", contractAddress, "blockNumber", blockNumber).
		Debug("UpdateStorage")

	oldStorage := s.GetStorage(contractAddress, blockNumber)
	if oldStorage == nil {
		// notest
		s.StoreStorage(contractAddress, blockNumber, storage)
	} else {
		oldStorage.Update(storage)
		s.StoreStorage(contractAddress, blockNumber, oldStorage)
	}
}
