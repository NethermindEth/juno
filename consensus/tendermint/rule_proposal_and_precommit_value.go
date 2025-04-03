package tendermint

/*
Check the upon condition on line 49:

		49: upon {PROPOSAL, h_p, r, v, *} from proposer(h_p, r) AND 2f + 1 {PRECOMMIT, h_p, r, id(v)} while decision_p[h_p] = nil do
		50: 	if valid(v) then
		51: 		decisionp[hp] = v
		52: 		h_p ← h_p + 1
		53: 		reset lockedRound_p, lockedValue_p,	validRound_p and validValue_p to initial values and empty message log
		54: 		StartRound(0)

	 There is no need to check decision_p[h_p] = nil since it is implied that decision are made
	 sequentially, i.e. x, x+1, x+2... . The validity of the proposal value can be checked in the same if
	 statement since there is no else statement.
*/
func (t *Tendermint[V, H, A]) line49WhenProposalIsReceived(p Proposal[V, H, A], vID H, validProposal, proposalFromProposer bool) bool {
	precommits, hasQuorum := t.checkForQuorumPrecommit(p.R, vID)

	if validProposal && proposalFromProposer && hasQuorum {
		// After committing the block, how the new height and round is started needs to be coordinated
		// with the synchronisation process.
		t.blockchain.Commit(t.state.h, *p.Value, precommits)

		t.messages.deleteHeightMessages(t.state.h)
		t.state.h++
		t.startRound(0)

		return true
	}
	return false
}

/*
Check the upon condition on line 49:

	49: upon {PROPOSAL, h_p, r, v, *} from proposer(h_p, r) AND 2f + 1 {PRECOMMIT, h_p, r, id(v)} while decision_p[h_p] = nil do
	50: 	if valid(v) then
	51: 		decisionp[hp] = v
	52: 		h_p ← h_p + 1
	53: 		reset lockedRound_p, lockedValue_p,	validRound_p and validValue_p to initial values and empty message log
	54: 		StartRound(0)

Fetching the relevant proposal implies the sender of the proposal was the proposer for that
height and round. Also, since only the proposals with valid value are added to the message set, the
validity of the proposal can be skipped.

There is no need to check decision_p[h_p] = nil since it is implied that decision are made
sequentially, i.e. x, x+1, x+2... .
*/
func (t *Tendermint[V, H, A]) line49WhenPrecommitIsReceived(p Precommit[H, A]) bool {
	if p.ID != nil {
		proposal := t.findMatchingProposal(p.R, *p.ID)

		precommits, hasQuorum := t.checkForQuorumPrecommit(p.R, *p.ID)

		if proposal != nil && hasQuorum {
			t.blockchain.Commit(t.state.h, *proposal.Value, precommits)

			t.messages.deleteHeightMessages(t.state.h)
			t.state.h++
			t.startRound(0)

			return true
		}
	}
	return false
}
