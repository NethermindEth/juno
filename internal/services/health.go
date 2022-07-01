package services

import (
	"context"
	"fmt"

	"github.com/NethermindEth/juno/internal/config"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/feeder"
)

// In this package we can query that the other services are running

var HealthService healthService

// The health service contains the service information and the response
type healthService struct {
	service
	healthResponse
}

// Create a health response
type healthResponse struct {
	GatewayUp  bool   `json:"gateway_up"`
	RPCUp      bool   `json:"rpc_up"`
	SyncStatus string `json:"sync_status"`
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

	// Check if the feeder gateway is down
	if s.GatewayDown() {
		// notest
		s.logger.Panic("feeder gateway is down")
	}
	s.healthResponse.GatewayUp = true

	// Check if the RPC service is down
	if s.RPCDown() {
		// notest
		s.logger.Panic("rpc service is down")
	}
	s.healthResponse.RPCUp = true

	// TODO: Use the sync service to check status
	// Assume that sync service is complete
	s.healthResponse.SyncStatus = "syncing"

	// Pretty print the JSON response
	s.logger.Desugar().Info(fmt.Sprintf("%+v", s.healthResponse))

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
	_, err := client.GetBlock("", "latest")
	return err != nil
}

// Ask the RPC service if it is running
func (s *healthService) RPCDown() bool {
	// Asssert that the RPC service is running
	return false
}

// We should make an RPC implementation (async API) for this
