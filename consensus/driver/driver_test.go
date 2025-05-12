package driver_test

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/consensus/driver"
	"github.com/NethermindEth/juno/consensus/mocks"
	"github.com/NethermindEth/juno/consensus/tendermint"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

const (
	seed        = 20250509 // The day we write this test
	actionCount = 10
)

type expectedBroadcast struct {
	proposals  []types.Proposal[value, felt.Felt, felt.Felt]
	prevotes   []types.Prevote[felt.Felt, felt.Felt]
	precommits []types.Precommit[felt.Felt, felt.Felt]
}

type value uint64

func (v value) Hash() felt.Felt {
	return *new(felt.Felt).SetUint64(uint64(v))
}

type mockListener[M types.Message[value, felt.Felt, felt.Felt]] struct {
	ch chan M
}

func newMockListener[M types.Message[value, felt.Felt, felt.Felt]](ch chan M) *mockListener[M] {
	return &mockListener[M]{
		ch: ch,
	}
}

func (m *mockListener[M]) Listen() <-chan M {
	return m.ch
}

type mockBroadcaster[M types.Message[value, felt.Felt, felt.Felt]] struct {
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
	proposalCh chan types.Proposal[value, felt.Felt, felt.Felt],
	prevoteCh chan types.Prevote[felt.Felt, felt.Felt],
	precommitCh chan types.Precommit[felt.Felt, felt.Felt],
) driver.Listeners[value, felt.Felt, felt.Felt] {
	return driver.Listeners[value, felt.Felt, felt.Felt]{
		ProposalListener:  newMockListener(proposalCh),
		PrevoteListener:   newMockListener(prevoteCh),
		PrecommitListener: newMockListener(precommitCh),
	}
}

func mockBroadcasters() driver.Broadcasters[value, felt.Felt, felt.Felt] {
	return driver.Broadcasters[value, felt.Felt, felt.Felt]{
		ProposalBroadcaster:  &mockBroadcaster[types.Proposal[value, felt.Felt, felt.Felt]]{},
		PrevoteBroadcaster:   &mockBroadcaster[types.Prevote[felt.Felt, felt.Felt]]{},
		PrecommitBroadcaster: &mockBroadcaster[types.Precommit[felt.Felt, felt.Felt]]{},
	}
}

func mockTimeoutFn(step types.Step, round types.Round) time.Duration {
	return 1 * time.Millisecond
}

func getRandMessageHeader(random *rand.Rand) types.MessageHeader[felt.Felt] {
	return types.MessageHeader[felt.Felt]{
		Height: types.Height(random.Uint32()),
		Round:  types.Round(random.Int()),
		Sender: felt.FromUint64(random.Uint64()),
	}
}

func getRandProposal(random *rand.Rand) types.Proposal[value, felt.Felt, felt.Felt] {
	return types.Proposal[value, felt.Felt, felt.Felt]{
		MessageHeader: getRandMessageHeader(random),
		Value:         utils.HeapPtr(value(random.Uint64())),
		ValidRound:    types.Round(random.Int()),
	}
}

func getRandPrevote(random *rand.Rand) types.Prevote[felt.Felt, felt.Felt] {
	return types.Prevote[felt.Felt, felt.Felt]{
		MessageHeader: getRandMessageHeader(random),
		ID:            utils.HeapPtr(felt.FromUint64(random.Uint64())),
	}
}

func getRandPrecommit(random *rand.Rand) types.Precommit[felt.Felt, felt.Felt] {
	return types.Precommit[felt.Felt, felt.Felt]{
		MessageHeader: getRandMessageHeader(random),
		ID:            utils.HeapPtr(felt.FromUint64(random.Uint64())),
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
) []types.Action[value, felt.Felt, felt.Felt] {
	actions := make([]types.Action[value, felt.Felt, felt.Felt], actionCount)
	for i := range actionCount {
		switch random.Int() % 3 {
		case 0:
			proposal := getRandProposal(random)
			expectedBroadcast.proposals = append(expectedBroadcast.proposals, proposal)
			actions[i] = utils.HeapPtr(tendermint.BroadcastProposal[value, felt.Felt, felt.Felt](proposal))
		case 1:
			prevote := getRandPrevote(random)
			expectedBroadcast.prevotes = append(expectedBroadcast.prevotes, prevote)
			actions[i] = utils.HeapPtr(tendermint.BroadcastPrevote[felt.Felt, felt.Felt](prevote))
		case 2:
			precommit := getRandPrecommit(random)
			expectedBroadcast.precommits = append(expectedBroadcast.precommits, precommit)
			actions[i] = utils.HeapPtr(tendermint.BroadcastPrecommit[felt.Felt, felt.Felt](precommit))
		}
	}
	return actions
}

func toAction(timeout types.Timeout) types.Action[value, felt.Felt, felt.Felt] {
	return utils.HeapPtr(tendermint.ScheduleTimeout(timeout))
}

func increaseBroadcasterWaitGroup[M types.Message[value, felt.Felt, felt.Felt]](
	expectedBroadcast []M,
	broadcaster driver.Broadcaster[M, value, felt.Felt, felt.Felt],
) {
	broadcaster.(*mockBroadcaster[M]).wg.Add(len(expectedBroadcast))
}

func waitAndAssertBroadcaster[M types.Message[value, felt.Felt, felt.Felt]](
	t *testing.T,
	expectedBroadcast []M,
	broadcaster driver.Broadcaster[M, value, felt.Felt, felt.Felt],
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

	proposalCh := make(chan types.Proposal[value, felt.Felt, felt.Felt])
	prevoteCh := make(chan types.Prevote[felt.Felt, felt.Felt])
	precommitCh := make(chan types.Precommit[felt.Felt, felt.Felt])
	broadcasters := mockBroadcasters()

	stateMachine := mocks.NewMockStateMachine[value, felt.Felt, felt.Felt](ctrl)
	driver := driver.New(stateMachine, mockListeners(proposalCh, prevoteCh, precommitCh), broadcasters, mockTimeoutFn)

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
