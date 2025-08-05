package sync_test

import (
	"context"
	"testing"
	"time"

	"github.com/NethermindEth/juno/consensus/db"
	"github.com/NethermindEth/juno/consensus/driver"
	"github.com/NethermindEth/juno/consensus/mocks"
	"github.com/NethermindEth/juno/consensus/p2p"
	"github.com/NethermindEth/juno/consensus/proposal"
	"github.com/NethermindEth/juno/consensus/starknet"
	consensusSync "github.com/NethermindEth/juno/consensus/sync"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/p2p/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type (
	listeners    = p2p.Listeners[starknet.Value, starknet.Hash, starknet.Address]
	broadcasters = p2p.Broadcasters[starknet.Value, starknet.Hash, starknet.Address]
	tendermintDB = db.TendermintDB[starknet.Value, starknet.Hash, starknet.Address]
	blockchain   = driver.Blockchain[starknet.Value, starknet.Hash]
)

type mockBlockchain struct {
	t *testing.T
}

func (m *mockBlockchain) Commit(height types.Height, value starknet.Value) {
}

func newMockBlockchain(t *testing.T) blockchain {
	return &mockBlockchain{
		t: t,
	}
}

func mockTimeoutFn(step types.Step, round types.Round) time.Duration {
	return 10 * time.Second
}

func newTendermintDB(t *testing.T) tendermintDB {
	t.Helper()
	dbPath := t.TempDir()
	pebbleDB, err := pebble.New(dbPath)
	require.NoError(t, err)

	return db.NewTendermintDB[starknet.Value, starknet.Hash, starknet.Address](pebbleDB, types.Height(0))
}

func TestSync(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, cancel := context.WithCancel(t.Context())

	stateMachine := mocks.NewMockStateMachine[starknet.Value, starknet.Hash, starknet.Address](ctrl)
	stateMachine.EXPECT().ReplayWAL().AnyTimes().Return() // ignore WAL replay logic here
	stateMachine.EXPECT().ProcessStart(types.Round(0)).Return([]types.Action[starknet.Value, starknet.Hash, starknet.Address]{})
	stateMachine.EXPECT().ProcessPrecommit(gomock.Any()).Times(3).Return([]types.Action[starknet.Value, starknet.Hash, starknet.Address]{})
	stateMachine.EXPECT().ProcessProposal(gomock.Any()).Return(
		[]types.Action[starknet.Value, starknet.Hash, starknet.Address]{},
	)

	proposalCh := make(chan starknet.Proposal)
	prevoteCh := make(chan starknet.Prevote)
	precommitCh := make(chan starknet.Precommit)
	p2p := newMockP2P(proposalCh, prevoteCh, precommitCh)

	driver := driver.New(
		utils.NewNopZapLogger(),
		newTendermintDB(t),
		stateMachine,
		newMockBlockchain(t),
		p2p,
		newMockProposer(),
		mockTimeoutFn,
	)

	go func() {
		require.NoError(t, driver.Run(ctx))
	}()

	mockInCh := make(chan sync.BlockBody)
	mockP2PSyncService := newMockP2PSyncService(mockInCh)
	proposalStore := proposal.ProposalStore[starknet.Hash]{}

	consensusSyncService := consensusSync.New(&mockP2PSyncService, proposalCh, precommitCh, getPrecommits, toValue, &proposalStore)

	block0 := getCommittedBlock()
	block0Hash := block0.Block.Hash
	valueHash := toValue(block0Hash).Hash()
	go func() {
		mockP2PSyncService.recieveBlockOverP2P(block0)
	}()

	go func() {
		time.Sleep(2 * time.Second)
		cancel()
	}()
	consensusSyncService.Run(ctx)                     // Driver should trigger stopSyncCh and shut this service down
	require.NotEmpty(t, proposalStore.Get(valueHash)) // Ensure the Driver sees the correct proposal
}

func TestShutdownOnError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, cancel := context.WithCancel(t.Context())

	proposalCh := make(chan starknet.Proposal)
	precommitCh := make(chan starknet.Precommit)

	mockInCh := make(chan sync.BlockBody)
	mockP2PSyncService := newMockP2PSyncService(mockInCh)
	proposalStore := proposal.ProposalStore[starknet.Hash]{}

	consensusSyncService := consensusSync.New(&mockP2PSyncService, proposalCh, precommitCh, getPrecommits, toValue, &proposalStore)
	cancel()
	consensusSyncService.Run(ctx)

	mockP2PSyncService.shouldTriggerErr()
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
