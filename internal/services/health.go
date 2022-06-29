package services

import (
	"context"

	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/db/block"
	"github.com/NethermindEth/juno/internal/health"
	"github.com/NethermindEth/juno/internal/log"
)

var HealthCheck healthService

type healthService struct {
	service
	manager *block.Manager
}

func (s *healthService) Run() error {
	if s.logger == nil {
		s.logger = log.Default.Named("Health Service")
	}

	if err := s.service.Run(); err != nil {
		return err
	}

	s.setDefaults()

	s.HealthCheck()

	return nil
}

func (s *healthService) setDefaults() error {
	if s.manager == nil {
		// notest
		env, err := db.GetMDBXEnv()
		if err != nil {
			return err
		}
		database, err := db.NewMDBXDatabase(env, "BLOCK")
		if err != nil {
			return err
		}
		s.manager = block.NewManager(database)
	}
	return nil
}

func (s *healthService) Close(ctx context.Context) {
	s.service.Close(ctx)
}

func (s *healthService) HealthCheck() bool {
	// Last block in the feeder gateway
	lastBlock, err := health.GetLastBlock()
	if err != nil {
		return false
	}

	log.Default.Info("\n\nLast block in the feeder gateway \n\n", lastBlock)
	return true

	// Last block in the database
	// localLastBlock, err := s.manager.GetLastBlock()
}
