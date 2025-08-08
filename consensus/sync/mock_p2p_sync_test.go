package sync_test

import (
	"context"

	"github.com/NethermindEth/juno/p2p/sync"
)

type mockP2PSyncService struct {
	blockCh chan sync.BlockBody
}

func newMockP2PSyncService(blockCh chan sync.BlockBody) mockP2PSyncService {
	return mockP2PSyncService{blockCh: blockCh}
}

func (m *mockP2PSyncService) receiveBlockOverP2P(block sync.BlockBody) {
	m.blockCh <- block
}

func (m *mockP2PSyncService) Listen() <-chan sync.BlockBody {
	return m.blockCh
}

func (m *mockP2PSyncService) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}
