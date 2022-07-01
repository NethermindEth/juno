package services

import (
	"context"

	"github.com/NethermindEth/juno/internal/config"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/feeder"
)

// In this package we can query that the other services are running

var HealthService healthService

type healthService struct {
	service
}

func (s *healthService) Setup() {
	if s.Running() {
		// notest
		s.logger.Panic("service is already running")
	}
}

func (s *healthService) Run() error {
	if s.logger == nil {
		s.logger = log.Default.Named("HealthService")
	}

	if err := s.service.Run(); err != nil {
		// notest
		return err
	}

	if s.GatewayDown() {
		// notest
		s.logger.Panic("feeder gateway is down")
	}

	s.logger.Info("\nGateway UP\n")

	return nil
}

// Close the service
func (s *healthService) Close(ctx context.Context) {
	s.service.Close(ctx)
}

// We can also query if the feeder gateway is UP
func (s *healthService) GatewayDown() bool {
	// Start up a client and query the feeder gateway
	// if the feeder gateway is up, return true
	// if the feeder gateway is down, return false

	feederUrl := config.Runtime.Starknet.FeederGateway
	client := feeder.NewClient(feederUrl, "/feeder_gateway", nil)
	block, err := client.GetBlock("", "latest")

	if block != nil && err == nil {
		return false
	}
	return false
}

// Ask the RPC service if it is running
func (s *healthService) RPCDown() bool {
}

// We should make an RPC implementation (async API) for this
