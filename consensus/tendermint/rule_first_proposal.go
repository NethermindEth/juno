package tendermint

import "github.com/NethermindEth/juno/consensus/types"

/*
Check the upon condition on line 22:

	22: upon {PROPOSAL, h_p, round_p, v, -1} from proposer(h_p, round_p) while step_p = propose do
	23: 	if valid(v) ∧ (lockedRound_p = −1 ∨ lockedValue_p = v) then
	24: 		broadcast {PREVOTE, h_p, round_p, id(v)}
	25: 	else
	26: 		broadcast {PREVOTE, h_p, round_p, nil}
	27:		step_p ← prevote

Since the value's id is expected to be unique the id can be used to compare the values.
*/
func (t *stateMachine[V, H, A]) uponFirstProposal(cachedProposal *CachedProposal[V, H, A]) bool {
	return cachedProposal.ValidRound == -1 && t.state.step == types.StepPropose
}

func (t *stateMachine[V, H, A]) doFirstProposal(cachedProposal *CachedProposal[V, H, A]) Action[V, H, A] {
	shouldVoteForValue := cachedProposal.Valid &&
		(t.state.lockedRound == -1 ||
			t.state.lockedValue != nil && (*t.state.lockedValue).Hash() == *cachedProposal.ID)

	var votedID *H
	if shouldVoteForValue {
		votedID = cachedProposal.ID
	}
	return t.setStepAndSendPrevote(votedID)
}
