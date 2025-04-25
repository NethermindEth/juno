package driver_test

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/consensus/driver"
	"github.com/NethermindEth/juno/consensus/mocks"
	"github.com/NethermindEth/juno/consensus/tendermint"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

type expectedBroadcast struct {
	proposals  []tendermint.Proposal[value, felt.Felt, felt.Felt]
	prevotes   []tendermint.Prevote[felt.Felt, felt.Felt]
	precommits []tendermint.Precommit[felt.Felt, felt.Felt]
}

type value uint64

func (v value) Hash() felt.Felt {
	return *new(felt.Felt).SetUint64(uint64(v))
}

type mockListener[M tendermint.Message[value, felt.Felt, felt.Felt]] struct {
	ch chan M
}

func newMockListener[M tendermint.Message[value, felt.Felt, felt.Felt]](ch chan M) *mockListener[M] {
	return &mockListener[M]{
		ch: ch,
	}
}

func (m *mockListener[M]) Listen() <-chan M {
	return m.ch
}

type mockBroadcaster[M tendermint.Message[value, felt.Felt, felt.Felt]] struct {
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

func (m *mockBroadcaster[M]) SendMsg(addr felt.Felt, msg M) {
}

func mockListeners(
	proposalCh chan tendermint.Proposal[value, felt.Felt, felt.Felt],
	prevoteCh chan tendermint.Prevote[felt.Felt, felt.Felt],
	precommitCh chan tendermint.Precommit[felt.Felt, felt.Felt],
) driver.Listeners[value, felt.Felt, felt.Felt] {
	return driver.Listeners[value, felt.Felt, felt.Felt]{
		ProposalListener:  newMockListener(proposalCh),
		PrevoteListener:   newMockListener(prevoteCh),
		PrecommitListener: newMockListener(precommitCh),
	}
}

func mockBroadcasters() driver.Broadcasters[value, felt.Felt, felt.Felt] {
	return driver.Broadcasters[value, felt.Felt, felt.Felt]{
		ProposalBroadcaster:  &mockBroadcaster[tendermint.Proposal[value, felt.Felt, felt.Felt]]{},
		PrevoteBroadcaster:   &mockBroadcaster[tendermint.Prevote[felt.Felt, felt.Felt]]{},
		PrecommitBroadcaster: &mockBroadcaster[tendermint.Precommit[felt.Felt, felt.Felt]]{},
	}
}

func mockTimeoutFn(step tendermint.Step, round tendermint.Round) time.Duration {
	return 1 * time.Millisecond
}

func getRandMessageHeader() tendermint.MessageHeader[felt.Felt] {
	return tendermint.MessageHeader[felt.Felt]{
		Height: tendermint.Height(rand.Uint32()),
		Round:  tendermint.Round(rand.Int()),
		Sender: felt.FromUint64(rand.Uint64()),
	}
}

func getRandProposal() tendermint.Proposal[value, felt.Felt, felt.Felt] {
	return tendermint.Proposal[value, felt.Felt, felt.Felt]{
		MessageHeader: getRandMessageHeader(),
		Value:         utils.HeapPtr(value(rand.Uint64())),
		ValidRound:    tendermint.Round(rand.Int()),
	}
}

func getRandPrevote() tendermint.Prevote[felt.Felt, felt.Felt] {
	return tendermint.Prevote[felt.Felt, felt.Felt]{
		MessageHeader: getRandMessageHeader(),
		ID:            utils.HeapPtr(felt.FromUint64(rand.Uint64())),
	}
}

func getRandPrecommit() tendermint.Precommit[felt.Felt, felt.Felt] {
	return tendermint.Precommit[felt.Felt, felt.Felt]{
		MessageHeader: getRandMessageHeader(),
		ID:            utils.HeapPtr(felt.FromUint64(rand.Uint64())),
	}
}

func getRandTimeout(step tendermint.Step) tendermint.Timeout {
	return tendermint.Timeout{
		Height: tendermint.Height(rand.Uint32()),
		Step:   step,
		Round:  tendermint.Round(rand.Int()),
	}
}

const (
	actionCount = 10
)

func registerActions(expectedBroadcast *expectedBroadcast) []tendermint.Action[value, felt.Felt, felt.Felt] {
	actions := make([]tendermint.Action[value, felt.Felt, felt.Felt], actionCount)
	for i := range actionCount {
		switch rand.Int() % 3 {
		case 0:
			proposal := getRandProposal()
			expectedBroadcast.proposals = append(expectedBroadcast.proposals, proposal)
			actions[i] = utils.HeapPtr(tendermint.BroadcastProposal[value, felt.Felt, felt.Felt](proposal))
		case 1:
			prevote := getRandPrevote()
			expectedBroadcast.prevotes = append(expectedBroadcast.prevotes, prevote)
			actions[i] = utils.HeapPtr(tendermint.BroadcastPrevote[felt.Felt, felt.Felt](prevote))
		case 2:
			precommit := getRandPrecommit()
			expectedBroadcast.precommits = append(expectedBroadcast.precommits, precommit)
			actions[i] = utils.HeapPtr(tendermint.BroadcastPrecommit[felt.Felt, felt.Felt](precommit))
		}
	}
	return actions
}

func toAction(timeout tendermint.Timeout) tendermint.Action[value, felt.Felt, felt.Felt] {
	return utils.HeapPtr(tendermint.ScheduleTimeout(timeout))
}

func requireBroadcast[M tendermint.Message[value, felt.Felt, felt.Felt]](
	expectedBroadcast []M,
	broadcaster driver.Broadcaster[M, value, felt.Felt, felt.Felt],
) {
	broadcaster.(*mockBroadcaster[M]).wg.Add(len(expectedBroadcast))
}

func assertBroadcast[M tendermint.Message[value, felt.Felt, felt.Felt]](
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
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedBroadcast := &expectedBroadcast{}

	proposalCh := make(chan tendermint.Proposal[value, felt.Felt, felt.Felt])
	prevoteCh := make(chan tendermint.Prevote[felt.Felt, felt.Felt])
	precommitCh := make(chan tendermint.Precommit[felt.Felt, felt.Felt])
	broadcasters := mockBroadcasters()

	stateMachine := mocks.NewMockStateMachine[value, felt.Felt, felt.Felt](ctrl)
	driver := driver.New(stateMachine, mockListeners(proposalCh, prevoteCh, precommitCh), broadcasters, mockTimeoutFn)

	inputTimeoutProposal := getRandTimeout(tendermint.StepPropose)
	inputTimeoutPrevote := getRandTimeout(tendermint.StepPrevote)
	inputTimeoutPrecommit := getRandTimeout(tendermint.StepPrecommit)

	inputProposalMsg := getRandProposal()
	inputPrevoteMsg := getRandPrevote()
	inputPrecommitMsg := getRandPrecommit()

	stateMachine.EXPECT().ProcessStart(tendermint.Round(0)).Return(append(registerActions(expectedBroadcast), toAction(inputTimeoutProposal)))
	stateMachine.EXPECT().ProcessProposal(inputProposalMsg).Return(append(registerActions(expectedBroadcast), toAction(inputTimeoutPrevote)))
	stateMachine.EXPECT().ProcessPrevote(inputPrevoteMsg).Return(append(registerActions(expectedBroadcast), toAction(inputTimeoutPrecommit)))
	stateMachine.EXPECT().ProcessPrecommit(inputPrecommitMsg).Return(nil)
	stateMachine.EXPECT().ProcessTimeout(inputTimeoutProposal).Return(registerActions(expectedBroadcast))
	stateMachine.EXPECT().ProcessTimeout(inputTimeoutPrevote).Return(registerActions(expectedBroadcast))
	stateMachine.EXPECT().ProcessTimeout(inputTimeoutPrecommit).Return(registerActions(expectedBroadcast))

	requireBroadcast(expectedBroadcast.proposals, broadcasters.ProposalBroadcaster)
	requireBroadcast(expectedBroadcast.prevotes, broadcasters.PrevoteBroadcaster)
	requireBroadcast(expectedBroadcast.precommits, broadcasters.PrecommitBroadcaster)

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

	assertBroadcast(t, expectedBroadcast.proposals, broadcasters.ProposalBroadcaster)
	assertBroadcast(t, expectedBroadcast.prevotes, broadcasters.PrevoteBroadcaster)
	assertBroadcast(t, expectedBroadcast.precommits, broadcasters.PrecommitBroadcaster)

	driver.Stop()
}
