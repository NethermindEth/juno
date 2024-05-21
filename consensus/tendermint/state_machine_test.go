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
	// a bit tricky might need to make timeout callback function a dependency
}

// test machine creation
func TestStateMachineCreation(t *testing.T) {

}

// test machine transitions (the bulk of the tests)

// Test naming convention.
// Test_KeyState_Messages-received_DecisionCondition_ExpectedAction_ExpectedResultingState

// get proposals
func TestNotInProposeStep_OnProposalFromProposer_DoNoBroadcast_NoStateChange(t *testing.T) {

}

func TestInProposeStep_OnProposalFromProposerWithReceivedNoPreviouslyValidRound_ValidProposedValueAndNoPreviouslyLockedRound_DoBroadcastPreVoteWithId(t *testing.T) {

}

func TestInProposeStep_OnProposalFromProposerWithReceivedNoPreviouslyValidRound_ValidProposedValueAndLockedValueMatchProposedValue_DoBroadcastPreVoteWithId(t *testing.T) {

}

func TestInProposeStep_OnProposalFromProposerWithReceivedNoPreviouslyValidRound_ValidProposedValueAndLockedValueDoesNotMatchProposedValueAndPreviouslyLockedRound_DoBroadcastPreVoteWithNoId(t *testing.T) {

}

func TestInProposeStep_OnProposalFromProposerWithReceivedNoPreviouslyValidRound_InvalidProposedValue_DoBroadcastPreVoteWithNoId(t *testing.T) {

}

func TestInProposeStep_OnProposalFromProposerWithReceivedNoPreviouslyValidRound_PreviouslyLockedRoundAndLockedValueDoesNotMatchProposedValue_DoBroadcastsPreVoteWithNoId(t *testing.T) {

}

func TestInProposeStep_OnProposalFromProposer_DoTransitionToPreVoteStep(t *testing.T) {

}

func TestInProposeStep_OnProposalFromProposer_AfterTransitionToPreVoteStep_OnlyStepStateValueChanges(t *testing.T) {

}

// proposal and with majority vote
func TestNotInProposeState__OnProposal_h_r_v_vr_FromProposerAndMajorityPreVote_h_vr_idv__DoNoBroadcast__NoStateChange(t *testing.T) {

}

func TestInProposeState__OnProposal_h_r_v_vr_FromProposerAndMajorityPreVote_h_vr_idv_WithReceivedNoPreviouslyValidRound__DoNoBroadcast__NoStateChange(t *testing.T) {

}

func TestInProposeState__OnProposal_h_r_v_vr_FromProposerAndMajorityPreVote_h_vr_idv_WithReceivedValidRoundGreaterThanCurrentValidRound__DoNoBroadcast__NoStateChange(t *testing.T) {

}

func TestInProposeState__OnProposal_h_r_v_vr_FromProposerAndMajorityPreVote_h_vr_idv_WithReceivedValidRoundIsValid__InvalidProposedValue__DoBroadcastPreVoteWithNoId(t *testing.T) {

}

func TestInProposeState__OnProposal_h_r_v_vr_FromProposerAndMajorityPreVote_h_vr_idv_WithReceivedValidRoundIsValid__LockedRoundGreaterThanReceivedValidRoundAndLockedValueDoesNotMatchProposedValue__DoBroadcastPreVoteWithNoValueId(t *testing.T) {

}

func TestInProposeState__OnProposal_h_r_v_vr_FromProposerAndMajorityPreVote_h_vr_idv_WithReceivedValidRoundIsValid__ValidProposedValueAndLockedRoundLessThanOrEqualToReceivedValidRound__DoBroadcastPreVoteWithValueId(t *testing.T) {

}

func TestInProposeState__OnProposal_h_r_v_vr_FromProposerAndMajorityPreVote_h_vr_idv_WithReceivedValidRoundIsValid__ValidProposedValueAndLockedValueMatchesProposedValue__DoBroadcastPreVoteWithValueId(t *testing.T) {

}

func TestInProposeState__OnProposal_h_r_v_vr_FromProposerAndMajorityPreVote_h_vr_idv_WithReceivedValidRoundIsValid__DoTransitionToPreVoteStep(t *testing.T) {

}

func TestInProposeState__OnProposal_h_r_v_vr_FromProposerAndMajorityPreVote_h_vr_idv_WithReceivedValidRoundIsValid__AfterTransitionToPreVoteStep_OnlyStepStateValueChanges(t *testing.T) {

}
