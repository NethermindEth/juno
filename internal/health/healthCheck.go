package health

// This file checks if each of the processes running within Juno are live
import (
	"github.com/NethermindEth/juno/internal/config"
	"github.com/NethermindEth/juno/pkg/feeder"
)

// For a user, health check means:
// 0. Separate endpoint to query
// 1. Am I up to date with latest confirmed block?
// 2. Am I syncing?
// 3. Is the RPC server up?

func GetLastBlock() (string, error) {
	// Start a feeder gateway client and check last block
	feederUrl := config.Runtime.Starknet.FeederGateway
	client := feeder.NewClient(feederUrl, "/feeder_gateway", nil)
	block, err := client.GetBlock("", "latest")
	hash := block.BlockHash

	return hash, err
}
