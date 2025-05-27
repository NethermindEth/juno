package driver_test

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/consensus/driver"
	"github.com/NethermindEth/juno/consensus/mocks"
	"github.com/NethermindEth/juno/consensus/p2p"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

type (
	broadcaster[M starknet.Message] = p2p.Broadcaster[M, starknet.Value]
	listeners                       = p2p.Listeners[starknet.Value]
	broadcasters                    = p2p.Broadcasters[starknet.Value]
)

const (
	seed        = 20250509 // The day we write this test
	actionCount = 10
)

type expectedBroadcast struct {
	proposals  []starknet.Proposal
	prevotes   []types.Prevote
	precommits []types.Precommit
}

type mockListener[M starknet.Message] struct {
	ch chan M
}

func newMockListener[M starknet.Message](ch chan M) *mockListener[M] {
	return &mockListener[M]{
		ch: ch,
	}
}

func (m *mockListener[M]) Listen() <-chan M {
	return m.ch
}

type mockBroadcaster[M starknet.Message] struct {
	wg                  sync.WaitGroup
	mu                  sync.Mutex
	broadcastedMessages []M
}

func (m *mockBroadcaster[M]) Broadcast(msg M) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.broadcastedMessages = append(m.broadcastedMessages, msg)
	m.wg.Done()
}

func mockListeners(
	proposalCh chan starknet.Proposal,
	prevoteCh chan types.Prevote,
	precommitCh chan types.Precommit,
) listeners {
	return listeners{
		ProposalListener:  newMockListener(proposalCh),
		PrevoteListener:   newMockListener(prevoteCh),
		PrecommitListener: newMockListener(precommitCh),
	}
}

func mockBroadcasters() broadcasters {
	return broadcasters{
		ProposalBroadcaster:  &mockBroadcaster[starknet.Proposal]{},
		PrevoteBroadcaster:   &mockBroadcaster[types.Prevote]{},
		PrecommitBroadcaster: &mockBroadcaster[types.Precommit]{},
	}
}

func mockTimeoutFn(step types.Step, round types.Round) time.Duration {
	return 1 * time.Millisecond
}

func getRandMessageHeader(random *rand.Rand) types.MessageHeader {
	return types.MessageHeader{
		Height: types.Height(random.Uint32()),
		Round:  types.Round(random.Int()),
		Sender: types.Addr(felt.FromUint64(random.Uint64())),
	}
}

func getRandProposal(random *rand.Rand) starknet.Proposal {
	return starknet.Proposal{
		MessageHeader: getRandMessageHeader(random),
		Value:         utils.HeapPtr(starknet.Value(random.Uint64())),
		ValidRound:    types.Round(random.Int()),
	}
}

func getRandPrevote(random *rand.Rand) types.Prevote {
	return types.Prevote{
		MessageHeader: getRandMessageHeader(random),
		ID:            utils.HeapPtr(types.Hash(felt.FromUint64(random.Uint64()))),
	}
}

func getRandPrecommit(random *rand.Rand) types.Precommit {
	return types.Precommit{
		MessageHeader: getRandMessageHeader(random),
		ID:            utils.HeapPtr(types.Hash(felt.FromUint64(random.Uint64()))),
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
			actions[i] = utils.HeapPtr(types.BroadcastPrevote(prevote))
		case 2:
			precommit := getRandPrecommit(random)
			expectedBroadcast.precommits = append(expectedBroadcast.precommits, precommit)
			actions[i] = utils.HeapPtr(types.BroadcastPrecommit(precommit))
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

func TestDriver(t *testing.T) {
	random := rand.New(rand.NewSource(seed))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedBroadcast := &expectedBroadcast{}

	proposalCh := make(chan starknet.Proposal)
	prevoteCh := make(chan types.Prevote)
	precommitCh := make(chan types.Precommit)
	broadcasters := mockBroadcasters()

	stateMachine := mocks.NewMockStateMachine[starknet.Value](ctrl)
	stateMachine.EXPECT().ReplayWAL().AnyTimes().Return() // ignore WAL replay logic here
	driver := driver.New(memory.New(), stateMachine, mockListeners(proposalCh, prevoteCh, precommitCh), broadcasters, mockTimeoutFn)

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
	stateMachine.EXPECT().ProcessProposal(inputProposalMsg).Return(
		append(generateAndRegisterRandomActions(random, expectedBroadcast), toAction(inputTimeoutPrevote)),
	)
	stateMachine.EXPECT().ProcessPrevote(inputPrevoteMsg).Return(
		append(generateAndRegisterRandomActions(random, expectedBroadcast), toAction(inputTimeoutPrecommit)),
	)
	stateMachine.EXPECT().ProcessPrecommit(inputPrecommitMsg).Return(nil)
	stateMachine.EXPECT().ProcessTimeout(inputTimeoutProposal).Return(generateAndRegisterRandomActions(random, expectedBroadcast))
	stateMachine.EXPECT().ProcessTimeout(inputTimeoutPrevote).Return(generateAndRegisterRandomActions(random, expectedBroadcast))
	stateMachine.EXPECT().ProcessTimeout(inputTimeoutPrecommit).Return(generateAndRegisterRandomActions(random, expectedBroadcast))

	increaseBroadcasterWaitGroup(expectedBroadcast.proposals, broadcasters.ProposalBroadcaster)
	increaseBroadcasterWaitGroup(expectedBroadcast.prevotes, broadcasters.PrevoteBroadcaster)
	increaseBroadcasterWaitGroup(expectedBroadcast.precommits, broadcasters.PrecommitBroadcaster)

	driver.Start()

	go func() {
		proposalCh <- inputProposalMsg
	}()
	go func() {
		prevoteCh <- inputPrevoteMsg
	}()
	go func() {
		precommitCh <- inputPrecommitMsg
	}()

	waitAndAssertBroadcaster(t, expectedBroadcast.proposals, broadcasters.ProposalBroadcaster)
	waitAndAssertBroadcaster(t, expectedBroadcast.prevotes, broadcasters.PrevoteBroadcaster)
	waitAndAssertBroadcaster(t, expectedBroadcast.precommits, broadcasters.PrecommitBroadcaster)

	driver.Stop()
}
