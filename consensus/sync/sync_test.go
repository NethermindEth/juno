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
	"github.com/NethermindEth/juno/consensus/tendermint"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/pebblev2"
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

func TestSync(t *testing.T) {
	ctrl := gomock.NewController(t)

	ctx, cancel := context.WithCancel(t.Context())

	nodeAddr := felt.FromUint64[starknet.Address](123)
	logger := utils.NewNopZapLogger()

	dbPath := t.TempDir()
	testDB, err := pebblev2.New(dbPath)
	require.NoError(t, err)

	tmDB := db.NewTendermintDB[starknet.Value, starknet.Hash, starknet.Address](testDB)
	require.NotNil(t, tmDB)

	proposalStore := proposal.ProposalStore[starknet.Hash]{}
	allNodes := newNodes(4)
	mockApp := mocks.NewMockApplication[starknet.Value, starknet.Hash](ctrl)
	mockApp.EXPECT().Valid(gomock.Any()).AnyTimes().Return(true)
	mockCommitListener := mocks.NewMockCommitListener[starknet.Value, starknet.Hash](ctrl)

	mockCommitListener.EXPECT().
		OnCommit(gomock.Any(), types.Height(0), gomock.Any()).
		Do(func(_ any, newHeight types.Height, _ any) {
			comittedHeight = int(newHeight)
			cancel()
		})

	stateMachine := tendermint.New(logger, nodeAddr, mockApp, allNodes, types.Height(0))

	proposalCh := make(chan *starknet.Proposal)
	prevoteCh := make(chan *starknet.Prevote)
	precommitCh := make(chan *starknet.Precommit)

	listeners := listeners{
		ProposalListener:  newMockListener(proposalCh),
		PrevoteListener:   newMockListener(prevoteCh),
		PrecommitListener: newMockListener(precommitCh),
	}
	broadcasters := broadcasters{
		ProposalBroadcaster:  &mockBroadcaster[*starknet.Proposal]{},
		PrevoteBroadcaster:   &mockBroadcaster[*starknet.Prevote]{},
		PrecommitBroadcaster: &mockBroadcaster[*starknet.Precommit]{},
	}

	driver := driver.New(
		utils.NewNopZapLogger(),
		tmDB,
		stateMachine,
		mockCommitListener,
		broadcasters,
		listeners,
		mockTimeoutFn,
	)

	go func() {
		require.NoError(t, driver.Run(ctx))
	}()

	mockInCh := make(chan sync.BlockBody)

	consensusSyncService := consensusSync.New(mockInCh, proposalCh, precommitCh, getPrecommits, toValue, &proposalStore)

	block0 := getCommittedBlock(allNodes)
	block0Hash := block0.Block.Hash
	valueHash := toValue(block0Hash).Hash()
	go func() {
		// P2P Sync service inserts block into mockInCh
		mockInCh <- block0
	}()

	require.NoError(t, consensusSyncService.Run(ctx))
	require.NotEmpty(t, proposalStore.Get(valueHash)) // Ensure the Driver sees the correct proposal
	require.NotEqual(t, comittedHeight, -1, "expected a block to be committed")
}

func getCommittedBlock(allNodes nodes) sync.BlockBody {
	proposerAddr := allNodes.Proposer(types.Height(0), types.Round(0))
	return sync.BlockBody{
		Block: &core.Block{
			Header: &core.Header{
				Hash:             felt.NewFromUint64[felt.Felt](blockID),
				TransactionCount: 2,
				EventCount:       3,
				SequencerAddress: (*felt.Felt)(&proposerAddr),
				Number:           0,
			},
		},
		StateUpdate: &core.StateUpdate{
			StateDiff: &core.StateDiff{},
		},
		NewClasses:  make(map[felt.Felt]core.ClassDefinition),
		Commitments: &core.BlockCommitments{},
	}
}

func toValue(in *felt.Felt) starknet.Value {
	return starknet.Value(*in)
}

func getPrecommits(*sync.BlockBody) []starknet.Precommit {
	blockID := felt.FromUint64[starknet.Hash](blockID)
	return []starknet.Precommit{
		{
			MessageHeader: types.MessageHeader[starknet.Address]{
				Height: types.Height(0),
				Round:  types.Round(0),
				Sender: felt.FromUint64[starknet.Address](1),
			},
			ID: &blockID,
		},
		{
			MessageHeader: types.MessageHeader[starknet.Address]{
				Height: types.Height(0),
				Round:  types.Round(0),
				Sender: felt.FromUint64[starknet.Address](2),
			},
			ID: &blockID,
		},
		{
			MessageHeader: types.MessageHeader[starknet.Address]{
				Height: types.Height(0),
				Round:  types.Round(0),
				Sender: felt.FromUint64[starknet.Address](3),
			},
			ID: &blockID,
		},
	}
}
