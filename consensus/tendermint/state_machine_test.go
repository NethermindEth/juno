package tendermint

import (
	consensus "github.com/NethermindEth/juno/consensus/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
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
			NewStateMachine(&gossiper, nil, &proposer)
		})
	})

	t.Run("Initial state with no proposer panics", func(t *testing.T) {

		require.Panics(t, func() {
			NewStateMachine(&gossiper, &decider, nil)
		})
	})

	t.Run("Initial state with no gossiper panics", func(t *testing.T) {

		require.Panics(t, func() {
			NewStateMachine(nil, &decider, &proposer)
		})
	})

	t.Run("creates state machine  successfully", func(t *testing.T) {
		sm := NewStateMachine(&gossiper, &decider, &proposer)
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

func getGossiper() *consensus.Gossiper {
	panic("implement me")
}

func getStateMachine(initialState *State, config *Config) *StateMachine {
	var decider consensus.Decider
	var proposer consensus.Proposer
	var gossiper consensus.Gossiper
	return newStateMachine(initialState, &gossiper, &decider, &proposer, config)
}

func TestTransitionAsProposer(t *testing.T) {
	// is proposer
	t.Run("On start round sm broadcasts correct value if it already exists", func(t *testing.T) {
		var value consensus.Proposable
		var state *State = NewStateBuilder(&State{}).SetValidValue(&value).Build()
		sm := getStateMachine(state, nil)
		go sm.Run()

		msg := (*sm.gossiper).SubmitMessage()
		assert.NotNil(t, msg)
	})

	t.Run("On start round sm creates and broadcasts correct value if it does not exists", func(t *testing.T) {
		sm := getStateMachine(nil, nil)
		go sm.Run()

		msg := (*sm.gossiper).SubmitMessage() // blocks unitll msg received or time out
		assert.NotNil(t, msg)
	})
}

func TestTransitionAsNonProposer(t *testing.T) {
	// is proposer
	t.Run("On start round sm broadcasts Nothing if value already exists", func(t *testing.T) {
		var value consensus.Proposable
		var state *State = NewStateBuilder(&State{}).SetValidValue(&value).Build()
		sm := getStateMachine(state, nil)
		go sm.Run()

		msg := (*sm.gossiper).SubmitMessage()
		assert.NotNil(t, msg)
	})

	t.Run("On start round sm broadcasts Nothing it does not exists", func(t *testing.T) {
		sm := getStateMachine(nil, nil)
		go sm.Run()

		msg := (*sm.gossiper).SubmitMessage() // blocks unitll msg received or time out
		assert.NotNil(t, msg)
	})

	// a bit tricky might need to make timeout callback function a dependency for handle message function
	// also timeout time is based on a function of the number of rounds so far.
	t.Run("On start round schedules timout callback function", func(t *testing.T) {
		sm := getStateMachine(nil, &Config{timeOutProposal: func(sm *StateMachine, height HeightType, round RoundType) {
			(*sm.gossiper).SubmitMessageForBroadcast("timeout")
		}})

		go sm.Run()

		msg := (*sm.gossiper).SubmitMessage() // blocks unitll msg received or time out
		assert.NotNil(t, msg)
		assert.Equal(t, "timeout", msg)
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
