package tendermint

/*
Check the upon condition on line 28:

	28: upon {PROPOSAL, h_p, round_p, v, vr} from proposer(h_p, round_p) AND 2f + 1 {PREVOTE,h_p, vr, id(v)} while
		step_p = propose ∧ (vr ≥ 0 ∧ vr < round_p) do
	29: if valid(v) ∧ (lockedRound_p ≤ vr ∨ lockedValue_p = v) then
	30: 	broadcast {PREVOTE, hp, round_p, id(v)}
	31: else
	32:  	broadcast {PREVOTE, hp, round_p, nil}
	33: step_p ← prevote
*/
func (t *stateMachine[V, H, A]) uponProposalAndPolkaPrevious(cachedProposal *CachedProposal[V, H, A]) bool {
	vr := cachedProposal.ValidRound
	hasQuorum := t.checkQuorumPrevotesGivenProposalVID(vr, *cachedProposal.ID)
	return hasQuorum &&
		t.state.step == StepPropose &&
		vr >= 0 &&
		vr < t.state.round
}

func (t *stateMachine[V, H, A]) doProposalAndPolkaPrevious(cachedProposal *CachedProposal[V, H, A]) Action[V, H, A] {
	var votedID *H
	shouldVoteForValue := cachedProposal.Valid &&
		(t.state.lockedRound <= cachedProposal.ValidRound ||
			t.state.lockedValue != nil && cachedProposal.ID != nil && (*t.state.lockedValue).Hash() == *cachedProposal.ID)
	if shouldVoteForValue {
		votedID = cachedProposal.ID
	}
	return t.setStepAndSendPrevote(votedID)
}
