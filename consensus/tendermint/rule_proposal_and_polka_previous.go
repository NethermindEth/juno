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

Ideally, the condition on line 28 would be checked in a single if statement, however,
this cannot be done because valid round needs to be non-nil before the prevotes are fetched.
*/
func (t *Tendermint[V, H, A]) line28WhenProposalIsReceived(vr round, proposalFromProposer bool,
	vID H, validProposal bool,
) {
	if vr != -1 && proposalFromProposer && t.state.s == propose && vr >= 0 && vr < t.state.r {
		hasQuorum := t.checkQuorumPrevotesGivenProposalVID(vr, vID)
		if hasQuorum {
			var votedID *H
			if validProposal && (t.state.lockedRound <= vr || (*t.state.lockedValue).Hash() == vID) {
				votedID = &vID
			}
			t.sendPrevote(votedID)
		}
	}
}

/*
Check the upon condition on  line 28:

	28: upon {PROPOSAL, h_p, round_p, v, vr} from proposer(h_p, round_p) AND 2f + 1 {PREVOTE,h_p, vr, id(v)} while
		step_p = propose ∧ (vr ≥ 0 ∧ vr < round_p) do
	29: if valid(v) ∧ (lockedRound_p ≤ vr ∨ lockedValue_p = v) then
	30: 	broadcast {PREVOTE, hp, round_p, id(v)}
	31: else
	32:  	broadcast {PREVOTE, hp, round_p, nil}
	33: step_p ← prevote

Fetching the relevant proposal implies the sender of the proposal was the proposer for that
height and round. Also, since only the proposals with valid value are added to the message set, the
validity of the proposal can be skipped.

Calculating quorum of prevotes is more resource intensive than checking other condition on line	28,
therefore, it is checked in a subsequent if statement.
*/
func (t *Tendermint[V, H, A]) line28WhenPrevoteIsReceived(p Prevote[H, A]) {
	// vr >= 0 doesn't need to be checked since vr is a uint
	if vr := p.R; p.ID != nil && t.state.s == propose && vr < t.state.r {
		proposal := t.findMatchingProposal(t.state.r, *p.ID)
		hasQuorum := t.checkQuorumPrevotesGivenProposalVID(p.R, *p.ID)

		if proposal != nil && hasQuorum {
			var votedID *H
			if t.state.lockedRound >= vr || (*t.state.lockedValue).Hash() == *p.ID {
				votedID = p.ID
			}
			t.sendPrevote(votedID)
		}
	}
}
