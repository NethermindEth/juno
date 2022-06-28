package health

// This file checks if each of the processes running within Juno are live
import (
	"github.com/NethermindEth/juno/internal/config"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/feeder"
)

// For a user, health check means:
// 1. Am I up to date with latest confirmed block?
// 2. Am I syncing?
// 3. Is the RPC server up?

func HealthCheck() error {
	log.Default.Info("\n \n Starting Health check\n")

	pingFeederGateway()
	checkSyncStatus()

	// Test each of the services, update healthy if they are alive
	return nil
}

func pingFeederGateway() error {
	// Start a feeder gateway client and check last block
	feederUrl := config.Runtime.Starknet.FeederGateway
	client := feeder.NewClient(feederUrl, "/feeder_gateway", nil)
	client.GetBlock("", "latest")
	return nil
}

func checkSyncStatus() error {
	// Check last block
	return nil
}
