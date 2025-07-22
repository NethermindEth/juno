package sync_test

import (
	"context"
	"errors"
	"testing"

	"github.com/NethermindEth/juno/consensus/driver"
	"github.com/NethermindEth/juno/consensus/mocks"
	"github.com/NethermindEth/juno/consensus/proposal"
	"github.com/NethermindEth/juno/consensus/starknet"
	consensusSync "github.com/NethermindEth/juno/consensus/sync"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type mockP2PSyncService struct {
	syncReceiveCh   chan sync.BlockBody          // blocks received over p2p go here
	blockListenerCh chan (<-chan sync.BlockBody) // Listener will get this
	triggerErr      bool
}

func newMockP2PSyncService(syncReceiveCh chan sync.BlockBody) mockP2PSyncService {
	return mockP2PSyncService{
		syncReceiveCh: syncReceiveCh,
	}
}

func (m *mockP2PSyncService) shouldTriggerErr(triggerErr bool) {
	m.triggerErr = triggerErr
}

func (m *mockP2PSyncService) recieveBlockOverP2P(block sync.BlockBody) {
	m.syncReceiveCh <- block
}

func (m *mockP2PSyncService) SetListener() {
	m.blockListenerCh = make(chan (<-chan sync.BlockBody))
}

func (m *mockP2PSyncService) Listen() <-chan sync.BlockBody {
	return <-m.blockListenerCh
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
			blocksCh := make(chan sync.BlockBody, 1)
			blocksCh <- committedBlock
			m.blockListenerCh <- blocksCh
		}
	}
}

func TestSync(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	stateMachine := mocks.NewMockStateMachine[starknet.Value, starknet.Hash, starknet.Address](ctrl)
	stateMachine.EXPECT().ReplayWAL().AnyTimes().Return() // ignore WAL replay logic here
	stateMachine.EXPECT().ProcessStart(types.Round(0)).Return([]types.Action[starknet.Value, starknet.Hash, starknet.Address]{})
	stateMachine.EXPECT().ProcessPrecommit(gomock.Any()).Times(3).Return([]types.Action[starknet.Value, starknet.Hash, starknet.Address]{})
	stateMachine.EXPECT().ProcessProposal(gomock.Any()).Return(
		[]types.Action[starknet.Value, starknet.Hash, starknet.Address]{&types.StopSync{}}, // Pretend we caught the chain head. Commit action ignored here.
	)

	proposalCh := make(chan starknet.Proposal)
	prevoteCh := make(chan starknet.Prevote)
	precommitCh := make(chan starknet.Precommit)
	broadcasters := mockBroadcasters()

	stopSyncCh := make(chan struct{})

	driver := driver.New(
		utils.NewNopZapLogger(),
		newTendermintDB(t),
		stateMachine,
		newMockBlockchain(t),
		mockListeners(proposalCh, prevoteCh, precommitCh),
		broadcasters,
		mockTimeoutFn,
		stopSyncCh,
	)
	driver.Start()
	blockCh := make(chan sync.BlockBody)
	mockInCh := make(chan sync.BlockBody)
	mockP2PSyncService := newMockP2PSyncService(mockInCh)
	proposalStore := proposal.ProposalStore[starknet.Hash]{}

	consensusSyncService := consensusSync.New(&mockP2PSyncService, proposalCh, precommitCh, getPrecommits, stopSyncCh, toValue, &proposalStore, blockCh)

	block0 := getCommittedBlock()
	block0Hash := block0.Block.Hash
	valueHash := toValue(block0Hash).Hash()
	go func() {
		mockP2PSyncService.recieveBlockOverP2P(block0)
	}()

	consensusSyncService.Run(ctx)                     // Driver should trigger stopSyncCh and shut this service down
	require.NotEmpty(t, proposalStore.Get(valueHash)) // Ensure the Driver sees the correct proposal
	_, stopSyncChIsOpen := <-stopSyncCh               // Ensure the Driver closed this channel after catching up to the chain head
	require.False(t, stopSyncChIsOpen)
}

func TestShutdownOnError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, cancel := context.WithCancel(t.Context())

	proposalCh := make(chan starknet.Proposal)
	precommitCh := make(chan starknet.Precommit)

	stopSyncCh := make(chan struct{})

	blockCh := make(chan sync.BlockBody)
	mockInCh := make(chan sync.BlockBody)
	mockP2PSyncService := newMockP2PSyncService(mockInCh)
	mockP2PSyncService.SetListener()
	proposalStore := proposal.ProposalStore[starknet.Hash]{}

	consensusSyncService := consensusSync.New(&mockP2PSyncService, proposalCh, precommitCh, getPrecommits, stopSyncCh, toValue, &proposalStore, blockCh)
	cancel()
	consensusSyncService.Run(ctx)

	mockP2PSyncService.shouldTriggerErr(true)
	consensusSyncService.Run(t.Context())
}

func getCommittedBlock() sync.BlockBody {
	return sync.BlockBody{
		Block: &core.Block{
			Header: &core.Header{
				Hash:             new(felt.Felt).SetUint64(1),
				TransactionCount: 2,
				EventCount:       3,
				SequencerAddress: new(felt.Felt).SetUint64(4),
				Number:           1,
			},
		},
		StateUpdate: &core.StateUpdate{
			StateDiff: &core.StateDiff{},
		},
		NewClasses:  make(map[felt.Felt]core.Class),
		Commitments: &core.BlockCommitments{},
	}
}

func toValue(in *felt.Felt) starknet.Value {
	return starknet.Value(*in)
}

func getPrecommits(types.Height) []types.Precommit[starknet.Hash, starknet.Address] {
	return []types.Precommit[starknet.Hash, starknet.Address]{
		// We don't use the round since it's not present in the spec yet
		{
			MessageHeader: types.MessageHeader[starknet.Address]{
				Height: types.Height(1),
				Sender: starknet.Address(*new(felt.Felt).SetUint64(1)),
			},
		},
		{
			MessageHeader: types.MessageHeader[starknet.Address]{
				Height: types.Height(1),
				Sender: starknet.Address(*new(felt.Felt).SetUint64(2)),
			},
		},
		{
			MessageHeader: types.MessageHeader[starknet.Address]{
				Height: types.Height(1),
				Sender: starknet.Address(*new(felt.Felt).SetUint64(3)),
			},
		},
	}
}
