package sync_test

import (
	"context"
	"testing"
	"time"

	"github.com/NethermindEth/juno/consensus/driver"
	"github.com/NethermindEth/juno/consensus/mocks"
	"github.com/NethermindEth/juno/consensus/p2p"
	"github.com/NethermindEth/juno/consensus/proposal"
	"github.com/NethermindEth/juno/consensus/starknet"
	consensusSync "github.com/NethermindEth/juno/consensus/sync"
	"github.com/NethermindEth/juno/consensus/tendermint"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var (
	comittedHeight = -1
	blockID        = uint64(9)
)

type (
	listeners    = p2p.Listeners[starknet.Value, starknet.Hash, starknet.Address]
	broadcasters = p2p.Broadcasters[starknet.Value, starknet.Hash, starknet.Address]
)

func mockTimeoutFn(step types.Step, round types.Round) time.Duration {
	return 10 * time.Second
}

func newDB(t *testing.T) *mocks.MockTendermintDB[starknet.Value, starknet.Hash, starknet.Address] {
	t.Helper()
	ctrl := gomock.NewController(t)
	// Ignore WAL for tests that use this
	db := mocks.NewMockTendermintDB[starknet.Value, starknet.Hash, starknet.Address](ctrl)
	db.EXPECT().GetWALEntries(gomock.Any()).AnyTimes()
	db.EXPECT().SetWALEntry(gomock.Any()).AnyTimes()
	db.EXPECT().Flush().AnyTimes()
	db.EXPECT().DeleteWALEntries(gomock.Any()).AnyTimes()
	return db
}

func TestSync(t *testing.T) {
	ctrl := gomock.NewController(t)

	ctx, cancel := context.WithCancel(t.Context())

	nodeAddr := starknet.Address(felt.FromUint64(123))
	logger := utils.NewNopZapLogger()
	tendermintDB := newDB(t)
	proposalStore := proposal.ProposalStore[starknet.Hash]{}
	allNodes := newNodes(4)
	mockApp := mocks.NewMockApplication[starknet.Value, starknet.Hash](ctrl)
	mockApp.EXPECT().Valid(gomock.Any()).AnyTimes().Return(true)
	mockCommitListener := mocks.NewMockCommitListener[starknet.Value, starknet.Hash, starknet.Hash](ctrl)

	mockCommitListener.EXPECT().
		Commit(gomock.Any(), types.Height(0), gomock.Any()).
		Do(func(_ any, newHeight types.Height, _ any) {
			comittedHeight = int(newHeight)
			cancel()
		})

	stateMachine := tendermint.New(tendermintDB, logger, nodeAddr, mockApp, allNodes, types.Height(0))

	proposalCh := make(chan starknet.Proposal)
	prevoteCh := make(chan starknet.Prevote)
	precommitCh := make(chan starknet.Precommit)
	p2p := newMockP2P(proposalCh, prevoteCh, precommitCh)

	driver := driver.New(
		utils.NewNopZapLogger(),
		tendermintDB,
		stateMachine,
		mockCommitListener,
		p2p,
		mockTimeoutFn,
	)

	go func() {
		require.NoError(t, driver.Run(ctx))
	}()

	mockInCh := make(chan sync.BlockBody)
	mockP2PSyncService := newMockP2PSyncService(mockInCh)

	consensusSyncService := consensusSync.New(mockP2PSyncService.Listen(), proposalCh, precommitCh, getPrecommits, toValue, &proposalStore)

	block0 := getCommittedBlock()
	block0Hash := block0.Block.Hash
	valueHash := toValue(block0Hash).Hash()
	go func() {
		mockP2PSyncService.receiveBlockOverP2P(block0)
	}()

	require.NoError(t, consensusSyncService.Run(ctx))
	require.NotEmpty(t, proposalStore.Get(valueHash)) // Ensure the Driver sees the correct proposal
	require.NotEqual(t, comittedHeight, -1, "expected a block to be committed")
}

func getCommittedBlock() sync.BlockBody {
	return sync.BlockBody{
		Block: &core.Block{
			Header: &core.Header{
				Hash:             new(felt.Felt).SetUint64(blockID),
				TransactionCount: 2,
				EventCount:       3,
				SequencerAddress: new(felt.Felt).SetUint64(5),
				Number:           0,
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

func getPrecommits(*sync.BlockBody) []types.Precommit[starknet.Hash, starknet.Address] {
	blockID := starknet.Hash(*new(felt.Felt).SetUint64(blockID))
	return []types.Precommit[starknet.Hash, starknet.Address]{
		{
			MessageHeader: types.MessageHeader[starknet.Address]{
				Height: types.Height(0),
				Round:  types.Round(0),
				Sender: starknet.Address(*new(felt.Felt).SetUint64(1)),
			},
			ID: &blockID,
		},
		{
			MessageHeader: types.MessageHeader[starknet.Address]{
				Height: types.Height(0),
				Round:  types.Round(0),
				Sender: starknet.Address(*new(felt.Felt).SetUint64(2)),
			},
			ID: &blockID,
		},
		{
			MessageHeader: types.MessageHeader[starknet.Address]{
				Height: types.Height(0),
				Round:  types.Round(0),
				Sender: starknet.Address(*new(felt.Felt).SetUint64(3)),
			},
			ID: &blockID,
		},
	}
}
