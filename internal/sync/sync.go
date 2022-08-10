package sync

import (
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
)

// TODO logging
// TODO the feeder package should export a set of interfaces that we can mock

type Syncer interface {
	Run(errChan chan error)
}

type SyncService struct {
	l1 Syncer
	l2 Syncer
}

func NewMainnetSyncService(backend bind.ContractBackend, feederClient *feeder.Client) (*SyncService, error) {
	l1, err := NewMainnetL1SyncService(backend)
	if err != nil {
		return nil, err
	}
	return &SyncService{
		l1: l1,
		l2: NewL2SyncService(feederClient),
	}, nil
}

func NewGoerliSyncService(backend bind.ContractBackend, feederClient *feeder.Client) (*SyncService, error) {
	l1, err := NewGoerliL1SyncService(backend)
	if err != nil {
		return nil, err
	}
	return &SyncService{
		l1: l1,
		l2: NewL2SyncService(feederClient),
	}, nil
}

func (s *SyncService) Run(errChan chan error) {
	go s.l1.Run(errChan)
	go s.l2.Run(errChan)
}
