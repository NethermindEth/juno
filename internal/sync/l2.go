package sync

import (
	"github.com/NethermindEth/juno/pkg/feeder"
)

type L2SyncService struct {
	feederClient *feeder.Client
}

func NewL2SyncService(feederClient *feeder.Client) *L2SyncService {
	return &L2SyncService{
		feederClient: feederClient,
	}
}

// TODO implement
func (s *L2SyncService) Run(errChan chan error) {}
