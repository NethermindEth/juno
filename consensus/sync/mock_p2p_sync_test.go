package sync_test

import (
	"context"
	"errors"

	"github.com/NethermindEth/juno/p2p/sync"
)

type mockP2PSyncService struct {
	syncReceiveCh   chan sync.BlockBody // blocks received over p2p go here
	blockListenerCh chan sync.BlockBody // Listener will get this
	triggerErr      bool
}

func newMockP2PSyncService(syncReceiveCh chan sync.BlockBody) mockP2PSyncService {
	return mockP2PSyncService{
		syncReceiveCh:   syncReceiveCh,
		blockListenerCh: make(chan sync.BlockBody),
	}
}

func (m *mockP2PSyncService) shouldTriggerErr(triggerErr bool) {
	m.triggerErr = triggerErr
}

func (m *mockP2PSyncService) recieveBlockOverP2P(block sync.BlockBody) {
	m.syncReceiveCh <- block
}

func (m *mockP2PSyncService) Listen() <-chan sync.BlockBody {
	return m.blockListenerCh
}

func (m *mockP2PSyncService) Run(ctx context.Context) error {
	if m.triggerErr {
		return errors.New("mock sync returned an error")
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case committedBlock := <-m.syncReceiveCh:
			m.blockListenerCh <- committedBlock
		}
	}
}
