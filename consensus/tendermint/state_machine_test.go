package tendermint

import (
	consensus "github.com/NethermindEth/juno/consensus/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
	"time"
)

// TODO:  refactor tests  into groups the golang way.

// Todo: need to specify that some messages must match current round and height
// test state creation

func TestStateMachineCreation(t *testing.T) {
	t.Parallel()

	var decider consensus.Decider
	var proposer consensus.Proposer
	var gossiper consensus.Gossiper

	t.Run("Initial state with no decider panics", func(t *testing.T) {

		require.Panics(t, func() {
			NewStateMachine(gossiper, nil, proposer)
		})
	})

	t.Run("Initial state with no proposer panics", func(t *testing.T) {

		require.Panics(t, func() {
			NewStateMachine(gossiper, decider, nil)
		})
	})

	t.Run("Initial state with no gossiper panics", func(t *testing.T) {

		require.Panics(t, func() {
			NewStateMachine(nil, decider, proposer)
		})
	})

	t.Run("creates state machine  successfully", func(t *testing.T) {
		sm := NewStateMachine(gossiper, decider, proposer)
		assert.NotNil(t, sm.proposer)
		assert.NotNil(t, sm.gossiper)
		assert.NotNil(t, sm.decider)
	})
}

// test state mutation

func getDecider() *consensus.Decider {
	panic("implement me")
}

func getProposer() *consensus.Proposer {
	panic("implement me")
}

func getGossiper() consensus.Gossiper {
	panic("implement me")
}

type gossiperMock struct {
	mock.Mock
	inValues  []interface{}
	outValues []interface{}
}

func newGossiperMock(bufSize int, outValues []interface{}) *gossiperMock {
	if bufSize <= 0 {
		bufSize = 1
	}
	return &gossiperMock{
		inValues:  make([]interface{}, 0, bufSize),
		outValues: outValues,
	}
}

func (gm *gossiperMock) SubmitMessageForBroadcast(msg interface{}) {
	gm.inValues = append(gm.inValues, msg)
}

func (gm *gossiperMock) GetSubmittedMessage() interface{} {
	if gm.inValues == nil || len(gm.inValues) == 0 {
		return nil
	}
	val := gm.inValues[0]
	gm.inValues = gm.inValues[1:]
	return val
}

func (gm *gossiperMock) ReceiveMessageFromBroadcast(msg interface{}) {
	panic("implement me")
}

func (gm *gossiperMock) GetReceivedMessage() interface{} {
	if gm.inValues == nil || len(gm.inValues) == 0 {
		return nil
	}
	val := gm.inValues[0]
	gm.inValues = gm.inValues[1:]
	return val
}

func (gm *gossiperMock) ClearReceive() {
	panic("implement me")
}
func (gm *gossiperMock) ClearSubmit() {
	panic("implement me")
}
func (gm *gossiperMock) ClearAll() {
	panic("implement me")
}

type deciderMock struct {
	mock.Mock
}

func (dm *deciderMock) SubmitDecision(decision consensus.Proposable, height HeightType) bool {
	args := dm.Called(decision, height)
	return args.Bool(0)
}

func (dm *deciderMock) GetDecision(height HeightType) interface{} {
	args := dm.Called(height)
	return args.Get(0)
}

type proposerMock struct {
	mock.Mock
}

func (pm *proposerMock) Proposer(height HeightType, round RoundType) interface{} {
	args := pm.Called(height, round)
	return args.Get(0)
}

func (pm *proposerMock) IsProposer(height HeightType, round RoundType) uint8 {
	args := pm.Called(height, round)
	return args.Get(0).(uint8)
}

func (pm *proposerMock) StrictIsProposer(height HeightType, round RoundType) bool {
	args := pm.Called(height, round)
	return args.Bool(0)
}

func (pm *proposerMock) Elect(height HeightType, round RoundType) bool {
	args := pm.Called(height, round)
	return args.Bool(0)
}

func (pm *proposerMock) Propose(height HeightType, round RoundType) consensus.Proposable {
	args := pm.Called(height, round)
	return args.Get(0).(consensus.Proposable)
}

type proposableMock struct {
	mock.Mock
}

func (prm *proposableMock) Id() consensus.IdType {
	args := prm.Called()
	return args.Get(0).(consensus.IdType)
}

func (prm *proposableMock) Value() consensus.Proposable {
	args := prm.Called()
	return args.Get(0).(consensus.Proposable)
}

func (prm *proposableMock) IsId() bool {
	args := prm.Called()
	return args.Bool(0)
}

func (prm *proposableMock) IsValue() bool {
	args := prm.Called()
	return args.Bool(0)
}

func (prm *proposableMock) IsValid() bool {
	args := prm.Called()
	return args.Bool(0)
}
func (prm *proposableMock) Equals(other interface{}) bool {
	args := prm.Called(other, prm)
	return args.Bool(0)
}
func (prm *proposableMock) EqualsTo(other consensus.Proposable) bool {
	args := prm.Called(other, prm)
	return args.Bool(0)
}

func getStateMachine(initialState *State, config *Config) *StateMachine {
	var decider consensus.Decider
	var proposer consensus.Proposer
	gossiper := newGossiperMock(3, make([]interface{}, 2))
	return newStateMachine(initialState, gossiper, decider, proposer, config)
}

func TestTransitionAsProposer(t *testing.T) {
	// is proposer
	t.Run("On Start round sm broadcasts correct value if it already exists", func(t *testing.T) {
		value := new(proposableMock)
		value.On("Id").Return(consensus.IdType(0))

		decider := new(deciderMock)

		proposer := new(proposerMock)
		proposer.On("StrictIsProposer", mock.Anything, mock.Anything).Return(true)
		//proposer.On()
		gossiper := newGossiperMock(1, nil)

		state := NewStateBuilder(&State{decider: decider}).SetValidValue(value).Build()
		sm := newStateMachine(state, gossiper, decider, proposer, nil)
		sm.Start()
		msg := sm.gossiper.GetSubmittedMessage()
		assert.NotNil(t, msg)
		assert.Equal(t, value.Id(), msg.(consensus.Proposable).Id())
	})

	t.Run("On Start round sm creates and broadcasts correct value if it does not exists", func(t *testing.T) {
		value := new(proposableMock)
		value.On("Id").Return(consensus.IdType(0))

		decider := new(deciderMock)

		proposer := new(proposerMock)
		proposer.On("StrictIsProposer", mock.Anything, mock.Anything).Return(true)
		proposer.On("Propose", mock.Anything, mock.Anything).Return(value)

		gossiper := newGossiperMock(1, nil)

		sm := newStateMachine(nil, gossiper, decider, proposer, nil)
		sm.Start()
		msg := sm.gossiper.GetSubmittedMessage()
		assert.NotNil(t, msg)
		assert.Equal(t, value.Id(), msg.(consensus.Proposable).Id())
	})
}

func TestTransitionAsNonProposer(t *testing.T) {
	// is not proposer
	t.Run("On Start round sm broadcasts Nothing if value already exists", func(t *testing.T) {
		value := new(proposableMock)
		value.On("Id").Return(consensus.IdType(0))

		decider := new(deciderMock)

		proposer := new(proposerMock)
		proposer.On("StrictIsProposer", mock.Anything, mock.Anything).Return(false)

		gossiper := newGossiperMock(1, nil)

		state := NewStateBuilder(&State{decider: decider}).SetValidValue(value).Build()

		sm := newStateMachine(state, gossiper, decider, proposer, nil)

		sm.Start()

		msg := sm.gossiper.GetSubmittedMessage()
		assert.Nil(t, msg)
	})

	t.Run("On Start round sm broadcasts Nothing it does not exists", func(t *testing.T) {
		value := new(proposableMock)
		value.On("Id").Return(consensus.IdType(0))

		decider := new(deciderMock)

		proposer := new(proposerMock)
		proposer.On("StrictIsProposer", mock.Anything, mock.Anything).Return(false)
		proposer.On("Propose", mock.Anything, mock.Anything).Return(value)

		gossiper := newGossiperMock(1, nil)

		sm := newStateMachine(nil, gossiper, decider, proposer, nil)
		sm.Start()
		msg := sm.gossiper.GetSubmittedMessage()

		assert.Nil(t, msg)
	})

	// a bit tricky might need to make timeout callback function a dependency for handle message function
	// with wait groups
	// also timeout time is based on a function of the number of rounds so far.
	t.Run("On Start round schedules timeout callback function", func(t *testing.T) {
		value := new(proposableMock)
		value.On("Id").Return(consensus.IdType(0))

		decider := new(deciderMock)

		proposer := new(proposerMock)
		proposer.On("StrictIsProposer", mock.Anything, mock.Anything).Return(false)
		proposer.On("Propose", mock.Anything, mock.Anything).Return(value)

		gossiper := newGossiperMock(1, nil)

		var wg sync.WaitGroup
		wg.Add(1)

		config := Config{
			timeOutProposal: func(sm *StateMachine, height HeightType, round RoundType) {
				wg.Done()
			},
		}

		sm := newStateMachine(nil, gossiper, decider, proposer, &config)

		consensus.CheckTimeOut("scheduled", 5*time.Second, 10*time.Second)

		sm.Start()
		wg.Wait()
	})
}

// test machine transitions (the bulk of the tests)

// Test naming convention.
// Test_KeyState_Messages-received_DecisionCondition_ExpectedAction_ExpectedResultingState

// get proposals
func TestNotInProposeStep__OnProposalFromProposer__DoNoBroadcast__NoStateChange(t *testing.T) {

}

func TestInProposeStep__OnProposalFromProposerWithReceivedNoPreviouslyValidRound__ValidProposedValueAndNoPreviouslyLockedRound__DoBroadcastPreVoteWithId(t *testing.T) {

}

func TestInProposeStep__OnProposalFromProposerWithReceivedNoPreviouslyValidRound__ValidProposedValueAndLockedValueMatchProposedValue__DoBroadcastPreVoteWithId(t *testing.T) {

}

func TestInProposeStep__OnProposalFromProposerWithReceivedNoPreviouslyValidRound__ValidProposedValueAndLockedValueDoesNotMatchProposedValueAndPreviouslyLockedRound__DoBroadcastPreVoteWithNoId(t *testing.T) {

}

func TestInProposeStep__OnProposalFromProposerWithReceivedNoPreviouslyValidRound__InvalidProposedValue__DoBroadcastPreVoteWithNoId(t *testing.T) {

}

func TestInProposeStep__OnProposalFromProposerWithReceivedNoPreviouslyValidRound__PreviouslyLockedRoundAndLockedValueDoesNotMatchProposedValue__DoBroadcastsPreVoteWithNoId(t *testing.T) {

}

func TestInProposeStep__OnProposalFromProposer__DoTransitionToPreVoteStep(t *testing.T) {

}

func TestInProposeStep__OnProposalFromProposer__AfterTransitionToPreVoteStep__OnlyStepStateValueChanges(t *testing.T) {

}

// proposal and with majority vote
func TestNotInProposeState__OnProposal_h_r_v_vr_FromProposerAndMajorityPreVote_h_vr_idv__DoNoBroadcast__NoStateChange(t *testing.T) {

}

func TestInProposeState__OnProposal_h_r_v_vr_FromProposerAndMajorityPreVote_h_vr_idv_WithReceivedNoPreviouslyValidRound__DoNoBroadcast__NoStateChange(t *testing.T) {

}

func TestInProposeState__OnProposal_h_r_v_vr_FromProposerAndMajorityPreVote_h_vr_idv_WithReceivedPreviouslyValidRoundGreaterThanCurrentValidRound__DoNoBroadcast__NoStateChange(t *testing.T) {

}

func TestInProposeState__OnProposal_h_r_v_vr_FromProposerAndMajorityPreVote_h_vr_idv_WithReceivedPreviouslyValidRoundIsValid__InvalidProposedValue__DoBroadcastPreVoteWithNoId(t *testing.T) {

}

func TestInProposeState__OnProposal_h_r_v_vr_FromProposerAndMajorityPreVote_h_vr_idv_WithReceivedPreviouslyValidRoundIsValid__LockedRoundGreaterThanReceivedValidRoundAndLockedValueDoesNotMatchProposedValue__DoBroadcastPreVoteWithNoValueId(t *testing.T) {

}

func TestInProposeState__OnProposal_h_r_v_vr_FromProposerAndMajorityPreVote_h_vr_idv_WithReceivedPreviouslyValidRoundIsValid__ValidProposedValueAndLockedRoundLessThanOrEqualToReceivedValidRound__DoBroadcastPreVoteWithValueId(t *testing.T) {

}

func TestInProposeState__OnProposal_h_r_v_vr_FromProposerAndMajorityPreVote_h_vr_idv_WithReceivedPreviouslyValidRoundIsValid__ValidProposedValueAndLockedValueMatchesProposedValue__DoBroadcastPreVoteWithValueId(t *testing.T) {

}

func TestInProposeState__OnProposal_h_r_v_vr_FromProposerAndMajorityPreVote_h_vr_idv_WithReceivedPreviouslyValidRoundIsValid__DoTransitionToPreVoteStep(t *testing.T) {

}

func TestInProposeState__OnProposal_h_r_v_vr_FromProposerAndMajorityPreVote_h_vr_idv_WithReceivedPreviouslyValidRoundIsValid__AfterTransitionToPreVoteStep_OnlyStepStateValueChanges(t *testing.T) {

}

func TestInPreVoteState_FirstTime__OnPreVote_hp_rp_AnyValueId__DoScheduleOnTimeOutPreVoteForTimeOutPreVote(t *testing.T) {

}

func TestInPreVoteState_FirstTime__OnPreVote_hp_rp_AnyValueId__DoScheduleOnTimeOutPreVoteForTimeOutPreVote__AfterTimeOutOnStateMatch__DoBroadcastPreCommit__TransitionToPreCommit(t *testing.T) {

}

func TestInPreVoteState_FirstTime__OnPreVote_hp_rp_AnyValueId__DoScheduleOnTimeOutPreVoteForTimeOutPreVote__AfterTimeOutOnStateDoesNotMatch__DoNothing__NoStateChange(t *testing.T) {

}

func TestInPreVoteState_NotFirstTime__OnPreVote_hp_rp_AnyValueId__DoNothing__NoStateChange(t *testing.T) {

}

func TestInPreVoteState__OnProposal_hp_rp_v_AnyPreviouslyValidRound_FromProposerAndMajorityPreVote_hp_rp_idv__ValidProposedValue__DoSetLockedValueAndLockedRoundAndValidValueAndValidRoundAndBroadcastPreCommit(t *testing.T) {

}

func TestInPreCommitState__OnProposal_hp_rp_v_AnyPreviouslyValidRound_FromProposerAndMajorityPreVote_hp_rp_idv__ValidProposedValue__DoSetValidValueAndValidRoundOnlyAndNoBroadcastAndNoStepChange(t *testing.T) {

}

func TestInProposeState__OnProposal_hp_rp_v_AnyPreviouslyValidRound_FromProposerAndMajorityPreVote_hp_rp_idv__ValidProposedValue__DoNothing(t *testing.T) {

}

func TestInPreVoteState__OnMajorityPreVote_hp_rp_NoValueId__DoBroadcastPreCommit__SetStepToPreCommit(t *testing.T) {

}

func TestInPreVoteState__OnLessThanMajorityPreVote_hp_rp_NoValueId__DoNothing(t *testing.T) {

}

func TestInAnyState__OnMajorityPreCommit_hp_rp_AnyValueId_FirstTime__DoScheduleOnTimeOutPreCommitForTimeOutPreCommit__AfterTimeOutOnStateMatch__DoStartNextRound(t *testing.T) {

}

func TestInAnyState__OnMajorityPreCommit_hp_rp_AnyValueId_FirstTime__DoScheduleOnTimeOutPreCommitForTimeOutPreCommit__AfterTimeOutOnStateDoesNotMatch__DoNothing(t *testing.T) {

}

func TestInAnyState__OnLessThanMajorityPreCommit_hp_rp_AnyValueId_FirstTime__DoNothing(t *testing.T) {

}

func TestInAnyState__OnProposal_hp_r_v_AnyPreviouslyValidRound_FromProposerAndMajorityPreCommit_hp_r_idv__NoDecisionForCurrentHeight_ValidProposedValue__DoSetDecisionIncreaseHeightResetStateEmptyMessageLogAndStartNewRound(t *testing.T) {

}

func TestInAnyState__OnProposal_hp_r_v_AnyPreviouslyValidRound_FromProposerAndMajorityPreCommit_hp_r_idv__NoDecisionForCurrentHeight_InValidProposedValue__DoNothing(t *testing.T) {

}

func TestInAnyState__OnProposal_hp_r_v_AnyPreviouslyValidRound_FromProposerAndLessThanMajorityPreCommit_hp_r_idv__DoNothing(t *testing.T) {

}

// join an ongoing round if node is behind
func TestInAnyState__OnMinorityAnyVote_hp_r_AnyValueORAnyId_AnyPreviouslyValidRound__And_r_GreaterThan_rp__DoStartNewRound_r(t *testing.T) {

}

func TestInAnyState__OnLessThanMinorityAnyVote_hp_r_AnyValueORAnyId_AnyPreviouslyValidRound__DoStartNewRound_r(t *testing.T) {
	// do nothing for r > rp and for r < rp
}

// Test Misc Functions
func TestInProposeState__OnTimeOutPropose__WithMatchingState__DoBroadcastPreVoteWithNoValueIdSetStepToPreVote(t *testing.T) {

}

func TestInProposeState__OnTimeOutPropose__WithNoMatchingState__DoNothing(t *testing.T) {

}

func TestInNonProposeState__OnTimeOutPropose__DoNothing(t *testing.T) {

}

func TestInPreVoteState__OnTimeOutPreVote__WithMatchingState__DoBroadcastPreCommitWithNoValueIdSetStepToPreCommit(t *testing.T) {

}

func TestInPreVoteState__OnTimeOutPreVote__WithNoMatchingState__DoNothing(t *testing.T) {

}

func TestInNonePreVoteState__OnTimeOutPreVote__DoNothing(t *testing.T) {

}

func TestInAnyState__OnTimeOutPreCommit__WithMatchingState__DoStartNextRound(t *testing.T) {

}

func TestInAnyState__OnTimeOutPreCommit__WithNoMatchingState__DoNothing(t *testing.T) {

}

func Test__Timeout__WithRound__DoReturnExpectedTimeoutValue(t *testing.T) {

}
