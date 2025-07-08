package driver_test

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/NethermindEth/juno/consensus/db"
	"github.com/NethermindEth/juno/consensus/driver"
	"github.com/NethermindEth/juno/consensus/mocks"
	"github.com/NethermindEth/juno/consensus/p2p"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/hash"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/utils"
	"github.com/sourcegraph/conc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type (
	broadcaster[M starknet.Message] = p2p.Broadcaster[M, starknet.Value, starknet.Hash, starknet.Address]
	listeners                       = p2p.Listeners[starknet.Value, starknet.Hash, starknet.Address]
	broadcasters                    = p2p.Broadcasters[starknet.Value, starknet.Hash, starknet.Address]
	tendermintDB                    = db.TendermintDB[starknet.Value, starknet.Hash, starknet.Address]
	commitListener                  = driver.CommitListener[starknet.Value, starknet.Hash, starknet.Address]
)

const (
	seed        = 20250509 // The day we write this test
	actionCount = 10
)

type expectedBroadcast struct {
	proposals  []starknet.Proposal
	prevotes   []starknet.Prevote
	precommits []starknet.Precommit
}

func mockTimeoutFn(step types.Step, round types.Round) time.Duration {
	return 1 * time.Millisecond
}

func getRandMessageHeader(random *rand.Rand) starknet.MessageHeader {
	return starknet.MessageHeader{
		Height: types.Height(random.Uint32()),
		Round:  types.Round(random.Int()),
		Sender: starknet.Address(felt.FromUint64(random.Uint64())),
	}
}

func getRandProposal(random *rand.Rand) starknet.Proposal {
	return starknet.Proposal{
		MessageHeader: getRandMessageHeader(random),
		Value:         utils.HeapPtr(starknet.Value(felt.FromUint64(random.Uint64()))),
		ValidRound:    types.Round(random.Int()),
	}
}

func getRandPrevote(random *rand.Rand) starknet.Prevote {
	return starknet.Prevote{
		MessageHeader: getRandMessageHeader(random),
		ID:            utils.HeapPtr(hash.Hash(felt.FromUint64(random.Uint64()))),
	}
}

func getRandPrecommit(random *rand.Rand) starknet.Precommit {
	return starknet.Precommit{
		MessageHeader: getRandMessageHeader(random),
		ID:            utils.HeapPtr(hash.Hash(felt.FromUint64(random.Uint64()))),
	}
}

func getRandTimeout(random *rand.Rand, step types.Step) types.Timeout {
	return types.Timeout{
		Height: types.Height(random.Uint32()),
		Step:   step,
		Round:  types.Round(random.Int()),
	}
}

// generateAndRegisterRandomActions generates a fixed number of randomised Tendermint actions.
// Each action is a broadcast proposal, prevote, or precommit, and is also
// recorded in the corresponding field of expectedBroadcast.
func generateAndRegisterRandomActions(
	random *rand.Rand,
	expectedBroadcast *expectedBroadcast,
) []starknet.Action {
	actions := make([]starknet.Action, actionCount)
	for i := range actionCount {
		switch random.Int() % 3 {
		case 0:
			proposal := getRandProposal(random)
			expectedBroadcast.proposals = append(expectedBroadcast.proposals, proposal)
			actions[i] = utils.HeapPtr(starknet.BroadcastProposal(proposal))
		case 1:
			prevote := getRandPrevote(random)
			expectedBroadcast.prevotes = append(expectedBroadcast.prevotes, prevote)
			actions[i] = utils.HeapPtr(starknet.BroadcastPrevote(prevote))
		case 2:
			precommit := getRandPrecommit(random)
			expectedBroadcast.precommits = append(expectedBroadcast.precommits, precommit)
			actions[i] = utils.HeapPtr(starknet.BroadcastPrecommit(precommit))
		}
	}
	return actions
}

func toAction(timeout types.Timeout) starknet.Action {
	return utils.HeapPtr(types.ScheduleTimeout(timeout))
}

func increaseBroadcasterWaitGroup[M starknet.Message](
	expectedBroadcast []M,
	broadcaster broadcaster[M],
) {
	broadcaster.(*mockBroadcaster[M]).wg.Add(len(expectedBroadcast))
}

func waitAndAssertBroadcaster[M starknet.Message](
	t *testing.T,
	expectedBroadcast []M,
	broadcaster broadcaster[M],
) {
	mockBroadcaster := broadcaster.(*mockBroadcaster[M])
	mockBroadcaster.wg.Wait()

	mockBroadcaster.mu.Lock()
	defer mockBroadcaster.mu.Unlock()

	assert.ElementsMatch(t, expectedBroadcast, mockBroadcaster.broadcastedMessages)
}

func newTendermintDB(t *testing.T) tendermintDB {
	t.Helper()
	dbPath := t.TempDir()
	pebbleDB, err := pebble.New(dbPath)
	require.NoError(t, err)

	return db.NewTendermintDB[starknet.Value, starknet.Hash, starknet.Address](pebbleDB, types.Height(0))
}

func TestDriver(t *testing.T) {
	random := rand.New(rand.NewSource(seed))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedBroadcast := &expectedBroadcast{}

	proposalCh := make(chan starknet.Proposal)
	prevoteCh := make(chan starknet.Prevote)
	precommitCh := make(chan starknet.Precommit)

	stateMachine := mocks.NewMockStateMachine[starknet.Value, hash.Hash, starknet.Address](ctrl)
	stateMachine.EXPECT().ReplayWAL().AnyTimes().Return() // ignore WAL replay logic here

	commitAction := starknet.Commit(getRandProposal(random))
	p2p := newMockP2P(proposalCh, prevoteCh, precommitCh)

	driver := driver.New(
		utils.NewNopZapLogger(),
		newTendermintDB(t),
		stateMachine,
		newMockCommitListener(t, &commitAction),
		p2p,
		mockTimeoutFn,
	)

	inputTimeoutProposal := getRandTimeout(random, types.StepPropose)
	inputTimeoutPrevote := getRandTimeout(random, types.StepPrevote)
	inputTimeoutPrecommit := getRandTimeout(random, types.StepPrecommit)

	inputProposalMsg := getRandProposal(random)
	inputPrevoteMsg := getRandPrevote(random)
	inputPrecommitMsg := getRandPrecommit(random)

	// The driver receives messages with random heights and rounds from the network [inputProposalMsg, inputPrevoteMsg, inputPrecommitMsg].
	// These will trigger `ProcessProposal`, `ProcessPrevote` and `ProcessPrecommit` in the stateMachine, which will cause
	// timeouts to be scheduled (`toAction`). These timeouts will then be triggered (`ProcessTimeout`).
	// We force the stateMachine to return a random set of actions (`generateAndRegisterRandomActions`) here just to test that
	// the driver will actually receive them.
	stateMachine.EXPECT().ProcessStart(types.Round(0)).Return(
		append(generateAndRegisterRandomActions(random, expectedBroadcast), toAction(inputTimeoutProposal)),
	)
	stateMachine.EXPECT().ProcessProposal(&inputProposalMsg).Return(
		append(generateAndRegisterRandomActions(random, expectedBroadcast), toAction(inputTimeoutPrevote)),
	)
	stateMachine.EXPECT().ProcessPrevote(&inputPrevoteMsg).Return(
		append(generateAndRegisterRandomActions(random, expectedBroadcast), toAction(inputTimeoutPrecommit)),
	)
	stateMachine.EXPECT().ProcessPrecommit(&inputPrecommitMsg).Return(
		append(generateAndRegisterRandomActions(random, expectedBroadcast), &commitAction),
	)
	stateMachine.EXPECT().ProcessStart(types.Round(0)).Return(nil)
	stateMachine.EXPECT().ProcessTimeout(inputTimeoutProposal).Return(generateAndRegisterRandomActions(random, expectedBroadcast))
	stateMachine.EXPECT().ProcessTimeout(inputTimeoutPrevote).Return(generateAndRegisterRandomActions(random, expectedBroadcast))
	stateMachine.EXPECT().ProcessTimeout(inputTimeoutPrecommit).Return(generateAndRegisterRandomActions(random, expectedBroadcast))

	increaseBroadcasterWaitGroup(expectedBroadcast.proposals, p2p.Broadcasters().ProposalBroadcaster)
	increaseBroadcasterWaitGroup(expectedBroadcast.prevotes, p2p.Broadcasters().PrevoteBroadcaster)
	increaseBroadcasterWaitGroup(expectedBroadcast.precommits, p2p.Broadcasters().PrecommitBroadcaster)

	ctx, cancel := context.WithCancel(t.Context())

	wg := conc.NewWaitGroup()
	wg.Go(func() {
		require.NoError(t, driver.Run(ctx))
	})
	wg.Go(func() {
		proposalCh <- inputProposalMsg
	})
	wg.Go(func() {
		prevoteCh <- inputPrevoteMsg
	})
	wg.Go(func() {
		precommitCh <- inputPrecommitMsg
	})
	t.Cleanup(wg.Wait)

	waitAndAssertBroadcaster(t, expectedBroadcast.proposals, p2p.Broadcasters().ProposalBroadcaster)
	waitAndAssertBroadcaster(t, expectedBroadcast.prevotes, p2p.Broadcasters().PrevoteBroadcaster)
	waitAndAssertBroadcaster(t, expectedBroadcast.precommits, p2p.Broadcasters().PrecommitBroadcaster)

	cancel()
}
