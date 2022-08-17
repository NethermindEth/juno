package sync

import (
	"fmt"

	syncdb "github.com/NethermindEth/juno/internal/db/sync"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/NethermindEth/juno/pkg/state"
	"github.com/ethereum/go-ethereum/ethclient"
)

// TODO
// - think about how to manage database to keep api clean (see erigon)
// - L2Backend interface
// - logging

type Syncer interface {
	Run(errChan chan error)
	Close() error
}

type SyncService struct {
	l1 Syncer
	l2 Syncer
}

func NewSyncService(network, nodeUrl, feederUrl string, syncManager *syncdb.Manager, stateManager state.StateManager) (*SyncService, error) {
	feederClient := feeder.NewClient(feederUrl, "/feeder_gateway", nil)
	l1Client, err := ethclient.Dial(nodeUrl)
	if err != nil {
		return nil, fmt.Errorf("connect to ethereum node: %w", err)
	}

	// Mainnet
	if network == "mainnet" {
		l1, err := NewMainnetL1SyncService(l1Client, syncManager, stateManager)
		if err != nil {
			return nil, err
		}
		return &SyncService{
			l1: l1,
			l2: NewL2SyncService(feederClient),
		}, nil
	}

	// Goerli
	l1, err := NewGoerliL1SyncService(l1Client, syncManager, stateManager)
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

func (s *SyncService) Close() error {
	if err := s.l1.Close(); err != nil {
		return err
	}
	return s.l2.Close()
}
