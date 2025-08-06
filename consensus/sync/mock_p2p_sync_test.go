package sync_test

import (
	"context"
	"errors"

	"github.com/NethermindEth/juno/p2p/sync"
)

type mockP2PSyncService struct {
	blockCh    chan sync.BlockBody
	triggerErr bool
}

func newMockP2PSyncService(blockCh chan sync.BlockBody) mockP2PSyncService {
	return mockP2PSyncService{blockCh: blockCh}
}

func (m *mockP2PSyncService) shouldTriggerErr() {
	m.triggerErr = true
}

func (m *mockP2PSyncService) recieveBlockOverP2P(block sync.BlockBody) {
	m.blockCh <- block
}

func (m *mockP2PSyncService) Listen() <-chan sync.BlockBody {
	return m.blockCh
}

func (m *mockP2PSyncService) Run(ctx context.Context) error {
	if m.triggerErr {
		return errors.New("mock sync returned an error")
	}
	<-ctx.Done()
	return nil
}
