package sync

import (
	"github.com/NethermindEth/juno/pkg/feeder"
)

type L2SyncService struct {
	// TODO this should be an interface
	// This will require refactor of feeder package
	backend *feeder.Client
}

func NewL2SyncService(backend *feeder.Client) *L2SyncService {
	return &L2SyncService{
		backend: backend,
	}
}

// TODO implement
func (s *L2SyncService) Run(errChan chan error) {}
