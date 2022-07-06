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

func (s *stateService) Setup(stateDb, binaryCodeDb, codeDefinitionDb db.Database) {
	if s.Running() {
		// notest
		s.logger.Panic("service is already running")
	}
	s.manager = state.NewStateManager(stateDb, binaryCodeDb, codeDefinitionDb)
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
		env, err := db.GetMDBXEnv()
		if err != nil {
			return err
		}
		stateDb, err := db.NewMDBXDatabase(env, "STATE")
		if err != nil {
			return err
		}
		binaryCodeDb, err := db.NewMDBXDatabase(env, "BINARY_CODE")
		if err != nil {
			return err
		}
		codeDefinitionDb, err := db.NewMDBXDatabase(env, "CODE_DEFINITION")
		if err != nil {
			return err
		}
		s.manager = state.NewStateManager(stateDb, binaryCodeDb, codeDefinitionDb)
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

func (s *stateService) StoreBinaryCode(contractAddress []byte, code *state.Code) {
	s.AddProcess()
	defer s.DoneProcess()

	s.logger.
		With("contractAddress", common.Bytes2Hex(contractAddress)).
		Debug("StoreBinaryCode")

	s.manager.PutBinaryCode(contractAddress, code)
}

func (s *stateService) GetBinaryCode(contractAddress []byte) *state.Code {
	s.AddProcess()
	defer s.DoneProcess()

	s.logger.
		With("contractAddress", contractAddress).
		Debug("GetCode")

	return s.manager.GetBinaryCode(contractAddress)
}

func (s *stateService) StoreCodeDefinition(contractHash []byte, codeDefinition *state.CodeDefinition) {
	s.AddProcess()
	defer s.DoneProcess()

	s.logger.
		With("contractHash", common.Bytes2Hex(contractHash)).
		Debug("StoreCodeDefinition")

	s.manager.PutCodeDefinition(contractHash, codeDefinition)
}

func (s *stateService) GetCodeDefinition(contractHash []byte) *state.CodeDefinition {
	s.AddProcess()
	defer s.DoneProcess()

	s.logger.
		With("contractHash", common.Bytes2Hex(contractHash)).
		Debug("GetCodeDefinition")

	return s.manager.GetCodeDefinition(contractHash)
}
