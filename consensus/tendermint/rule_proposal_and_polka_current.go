package tendermint

/*
Check upon condition on line 36:

	36: upon {PROPOSAL, h_p, round_p, v, ∗} from proposer(h_p, round_p) AND 2f + 1 {PREVOTE, h_p, round_p, id(v)} while
		valid(v) ∧ step_p ≥ prevote for the first time do
	37: if step_p = prevote then
	38: 	lockedValue_p ← v
	39: 	lockedRound_p ← round_p
	40: 	broadcast {PRECOMMIT, h_p, round_p, id(v))}
	41: 	step_p ← precommit
	42: validValue_p ← v
	43: validRound_p ← round_p

The condition on line 36 can should be checked in a single if statement, however,
checking for quroum is more resource intensive than other conditions, therefore, they are checked
first.
*/
func (t *Tendermint[V, H, A]) line36WhenProposalIsReceived(p Proposal[V, H, A], validProposal,
	proposalFromProposer bool, prevotesForHR map[A][]Prevote[H, A], vID H,
) {
	if validProposal && proposalFromProposer && !t.state.lockedValueAndOrValidValueSet && t.state.s >= prevote {
		var vals []A
		for addr, valPrevotes := range prevotesForHR {
			for _, v := range valPrevotes {
				if *v.ID == vID {
					vals = append(vals, addr)
				}
			}
		}

		if t.validatorSetVotingPower(vals) >= q(t.validators.TotalVotingPower(p.H)) {
			cr := t.state.r

			if t.state.s == prevote {
				t.state.lockedValue = p.Value
				t.state.lockedRound = cr
				t.sendPrecommit(&vID)
			}

			t.state.validValue = p.Value
			t.state.validRound = cr
			t.state.lockedValueAndOrValidValueSet = true
		}
	}
}

/*
Check upon condition on line 36:

	36: upon {PROPOSAL, h_p, round_p, v, ∗} from proposer(h_p, round_p) AND 2f + 1 {PREVOTE, h_p, round_p, id(v)} while
		valid(v) ∧ step_p ≥ prevote for the first time do
	37: if step_p = prevote then
	38: 	lockedValue_p ← v
	39: 	lockedRound_p ← round_p
	40: 	broadcast {PRECOMMIT, h_p, round_p, id(v))}
	41: 	step_p ← precommit
	42: validValue_p ← v
	43: validRound_p ← round_p

Fetching the relevant proposal implies the sender of the proposal was the proposer for that
height and round. Also, since only the proposals with valid value are added to the message set, the
validity of the proposal can be skipped.

Calculating quorum of prevotes is more resource intensive than checking other condition on line 36,
therefore, it is checked in a subsequent if statement.
*/
func (t *Tendermint[V, H, A]) line36WhenPrevoteIsReceived(p Prevote[H, A]) {
	if p.ID != nil && !t.state.lockedValueAndOrValidValueSet && t.state.s >= prevote {
		proposal := t.findMatchingProposal(t.state.r, *p.ID)
		hasQuorum := t.checkQuorumPrevotesGivenProposalVID(p.R, *p.ID)

		if proposal != nil && hasQuorum {
			cr := t.state.r

			if t.state.s == prevote {
				t.state.lockedValue = proposal.Value
				t.state.lockedRound = cr
				t.sendPrecommit(p.ID)
			}

			t.state.validValue = proposal.Value
			t.state.validRound = cr
			t.state.lockedValueAndOrValidValueSet = true
		}
	}
}
