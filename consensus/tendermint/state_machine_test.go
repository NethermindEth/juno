package tendermint

import "testing"

// Todo: need to specify that some messages must match current round and height
// test state creation

// test state mutation

func TestAsProposerStartRoundBroadCastsCurrentValueWhenValueAlreadyExists(t *testing.T) {

}

func TestAsProposerStartRoundBroadCastsCorrectMessageWhenValueAlreadyExists(t *testing.T) {

}

func TestAsProposerStartRoundBroadCastsCorrectMessageWhenValueAlreadyExistsAndNo(t *testing.T) {

}

func TestAsProposerStartRoundCreatesAndBroadCastsValueWhenValueDoesNotExists(t *testing.T) {

}

func TestAsProposerStartRoundBroadCastsCorrectMessageWhenValueDoesNotExists(t *testing.T) {

}

func TestAsNonProposerStartRoundBroadCastsNothingWhenValueDoesNotExists(t *testing.T) {

}

func TestAsNonProposerStartRoundBroadCastsNothingWhenValueExists(t *testing.T) {

}

func TestAsNonProposerStartRoundSchedulesTimeOut(t *testing.T) {
	// a bit tricky might need to make timeout callback function a dependency for handle message function
	// also timeout time is based on a function of the number of rounds so far.
}

// test machine creation
func TestStateMachineCreation(t *testing.T) {

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
