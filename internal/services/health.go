package services

import (
	"context"
	"fmt"

	"github.com/NethermindEth/juno/internal/config"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/feeder"
)

// TODO: FIgure out how to query other processes from healthprocess

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
	// notest
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
		s.healthResponse.GatewayUp = false
	} else {
		s.healthResponse.GatewayUp = true
	}

	// Check if the RPC service is down
	if s.RPCDown() {
		// notest
		s.healthResponse.RPCUp = false
	} else {
		s.healthResponse.RPCUp = true
	}

	// TODO: Use the sync service to check status
	// For now assume that sync service is running
	s.healthResponse.SyncStatus = "syncing"

	// Pretty print the JSON response
	s.logger.Desugar().Info(fmt.Sprintf("%+v", s.healthResponse))

	return nil
}

// Close the service
func (s *healthService) Close(ctx context.Context) {
	// notest
	s.service.Close(ctx)
}

// We can also query if the feeder gateway is UP
func (s *healthService) GatewayDown() bool {
	// notest
	// Start up a client and query the feeder gateway
	// We attempt to fetch the latest block

	feederUrl := config.Runtime.Starknet.FeederGateway
	client := feeder.NewClient(feederUrl, "/feeder_gateway", nil)
	_, err := client.GetBlock("", "latest")
	return err != nil
}

// Ask the RPC service if it is running
func (s *healthService) RPCDown() bool {
	// notest
	// Asssert that the RPC service is running
	return Handler.rpc.running
}

// We should make an RPC implementation (async API) for this
