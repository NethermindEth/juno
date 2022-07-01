package health

import (
	"fmt"

	"github.com/NethermindEth/juno/internal/config"
	"github.com/NethermindEth/juno/internal/db/block"
	"github.com/NethermindEth/juno/pkg/feeder"
)

// The health checker wraps both the internal database and the feeder gateway client
type HealthChecker struct {
	blockManager *block.Manager
	gateway      *feeder.Client
}

// Connect the health checker to both the block manager for default db and feeder gateway
func Setup(h *HealthChecker) {
	// Start a feeder gateway client and check last block
	feederUrl := config.Runtime.Starknet.FeederGateway
	h.gateway = feeder.NewClient(feederUrl, "/feeder_gateway", nil)

	// Connect to the normal database
	h.blockManager.NewManager(config.Runtime.Starknet.Database)
	fmt.Println(h.blockManager.GetBlockByNumber(10))
}

func RetrieveLastBlock(h *HealthChecker) *block.Block {
	return h.blockManager.GetLastBlock()
}
