package services

import (
	"context"

	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/db/state"
	. "github.com/NethermindEth/juno/internal/log"
)

var StateService stateService

type stateService struct {
	service
	manager *state.Manager
}

func (s *stateService) Setup(stateDb, codeDefinitionDb db.Database) {
	if s.Running() {
		// notest
		s.logger.Panic("service is already running")
	}
	s.manager = state.NewStateManager(stateDb, codeDefinitionDb)
}

func (s *stateService) Run() error {
	if s.logger == nil {
		s.logger = Logger.Named("StateService")
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
		codeDefinitionDb, err := db.NewMDBXDatabase(env, "CONTRACT_DEFINITION")
		if err != nil {
			return err
		}
		s.manager = state.NewStateManager(stateDb, codeDefinitionDb)
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
