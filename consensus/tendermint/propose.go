package tendermint

func (t *Tendermint[V, H, A]) handleProposal(p Proposal[V, H, A]) {
	if p.H < t.state.h {
		return
	}

	if !handleFutureHeightMessage(
		t,
		p,
		func(p Proposal[V, H, A]) height { return p.H },
		func(p Proposal[V, H, A]) round { return p.R },
		t.futureMessages.addProposal,
	) {
		return
	}

	if !handleFutureRoundMessage(t, p, func(p Proposal[V, H, A]) round { return p.R }, t.futureMessages.addProposal) {
		return
	}

	// The code below shouldn't panic because it is expected Proposal is well-formed. However, there need to be a way to
	// distinguish between nil and zero value. This is expected to be handled by the p2p layer.
	vID := (*p.Value).Hash()
	validProposal := t.application.Valid(*p.Value)
	proposalFromProposer := p.Sender == t.validators.Proposer(p.H, p.R)
	vr := p.ValidRound

	if validProposal {
		// Add the proposal to the message set even if the sender is not the proposer,
		// this is because of slahsing purposes
		t.messages.addProposal(p)
	}

	_, prevotesForHR, precommitsForHR := t.messages.allMessages(p.H, p.R)

	if t.line49WhenProposalIsReceived(p, precommitsForHR, vID, validProposal, proposalFromProposer) {
		return
	}

	if p.R < t.state.r {
		// Except line 49 all other upon condition which refer to the proposals expect to be acted upon
		// when the current round is equal to the proposal's round.
		return
	}

	t.line22(vr, proposalFromProposer, validProposal, vID)
	t.line28WhenProposalIsReceived(p, vr, proposalFromProposer, vID, validProposal)
	t.line36WhenProposalIsReceived(p, validProposal, proposalFromProposer, prevotesForHR, vID)
}

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
func (t *Tendermint[V, H, A]) line49WhenProposalIsReceived(p Proposal[V, H, A], precommitsForHR map[A][]Precommit[H,
	A], vID H, validProposal bool, proposalFromProposer bool,
) bool {
	precommits, vals := checkForQuorumPrecommit[H, A](precommitsForHR, vID)

	if validProposal && proposalFromProposer &&
		t.validatorSetVotingPower(vals) >= q(t.validators.TotalVotingPower(p.H)) {
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
Check the upon condition on line 22:

	22: upon {PROPOSAL, h_p, round_p, v, nil} from proposer(h_p, round_p) while step_p = propose do
	23: 	if valid(v) ∧ (lockedRound_p = −1 ∨ lockedValue_p = v) then
	24: 		broadcast {PREVOTE, h_p, round_p, id(v)}
	25: 	else
	26: 		broadcast {PREVOTE, h_p, round_p, nil}
	27:		step_p ← prevote

The implementation uses nil as -1 to avoid using int type.

Since the value's id is expected to be unique the id can be used to compare the values.
*/
func (t *Tendermint[V, H, A]) line22(vr int, proposalFromProposer, validProposal bool, vID H) {
	if vr == -1 && proposalFromProposer && t.state.s == propose {
		vote := Prevote[H, A]{
			H:      t.state.h,
			R:      t.state.r,
			ID:     nil,
			Sender: t.nodeAddr,
		}

		if validProposal && (t.state.lockedRound == -1 || (*t.state.lockedValue).Hash() == vID) {
			vote.ID = &vID
		}

		t.messages.addPrevote(vote)
		t.broadcasters.PrevoteBroadcaster.Broadcast(vote)
		t.state.s = prevote
	}
}

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
func (t *Tendermint[V, H, A]) line28WhenProposalIsReceived(p Proposal[V, H, A], vr int, proposalFromProposer bool,
	vID H, validProposal bool,
) {
	if vr != -1 && proposalFromProposer && t.state.s == propose && vr >= 0 && vr < int(t.state.r) {
		_, prevotesForHVr, _ := t.messages.allMessages(p.H, round(vr))

		vals := checkQuorumPrevotesGivenProposalVID(prevotesForHVr, vID)

		if t.validatorSetVotingPower(vals) >= q(t.validators.TotalVotingPower(p.H)) {
			vote := Prevote[H, A]{
				H:      t.state.h,
				R:      t.state.r,
				ID:     nil,
				Sender: t.nodeAddr,
			}

			if validProposal && (t.state.lockedRound <= vr || (*t.state.lockedValue).Hash() == vID) {
				vote.ID = &vID
			}

			t.messages.addPrevote(vote)
			t.broadcasters.PrevoteBroadcaster.Broadcast(vote)
			t.state.s = prevote
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
				t.state.lockedRound = int(cr)

				vote := Precommit[H, A]{
					H:      t.state.h,
					R:      t.state.r,
					ID:     &vID,
					Sender: t.nodeAddr,
				}

				t.messages.addPrecommit(vote)
				t.broadcasters.PrecommitBroadcaster.Broadcast(vote)
				t.state.s = precommit
			}

			t.state.validValue = p.Value
			t.state.validRound = int(cr)
			t.state.lockedValueAndOrValidValueSet = true
		}
	}
}
