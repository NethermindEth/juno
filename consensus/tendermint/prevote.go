package tendermint

import (
	"maps"
	"slices"
)

func (t *Tendermint[V, H, A]) handlePrevote(p Prevote[H, A]) {
	if p.H < t.state.h {
		return
	}

	if !handleFutureHeightMessage(
		t,
		p,
		func(p Prevote[H, A]) height { return p.H },
		func(p Prevote[H, A]) round { return p.R },
		t.futureMessages.addPrevote,
	) {
		return
	}

	if !handleFutureRoundMessage(t, p, func(p Prevote[H, A]) round { return p.R }, t.futureMessages.addPrevote) {
		return
	}

	t.messages.addPrevote(p)

	proposalsForHR, prevotesForHR, _ := t.messages.allMessages(p.H, p.R)

	t.line28WhenPrevoteIsReceived(p, prevotesForHR)

	if p.R == t.state.r {
		t.line34(p, prevotesForHR)
		t.line44(p, prevotesForHR)

		t.line36WhenPrevoteIsReceived(p, proposalsForHR, prevotesForHR)
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
func (t *Tendermint[V, H, A]) line28WhenPrevoteIsReceived(p Prevote[H, A], prevotesForHR map[A][]Prevote[H, A]) {
	// vr >= 0 doesn't need to be checked since vr is a uint
	if vr := p.R; p.ID != nil && t.state.s == propose && vr < t.state.r {
		cr := t.state.r

		proposalsForHCR, _, _ := t.messages.allMessages(p.H, cr)

		proposal := t.checkForMatchingProposalGivenPrevote(p, proposalsForHCR)
		vals := checkForQuorumPrevotesGivenPrevote(p, prevotesForHR)

		if proposal != nil && t.validatorSetVotingPower(vals) >= q(t.validators.TotalVotingPower(p.H)) {
			vote := Prevote[H, A]{
				H:      t.state.h,
				R:      t.state.r,
				ID:     nil,
				Sender: t.nodeAddr,
			}

			if t.state.lockedRound >= int(vr) || (*t.state.lockedValue).Hash() == *p.ID {
				vote.ID = p.ID
			}

			t.messages.addPrevote(vote)
			t.broadcasters.PrevoteBroadcaster.Broadcast(vote)
			t.state.s = prevote
		}
	}
}

/*
Check the upon condition on line 34:

	34: upon 2f + 1 {PREVOTE, h_p, round_p, ∗} while step_p = prevote for the first time do
	35: schedule OnTimeoutPrevote(h_p, round_p) to be executed after timeoutPrevote(round_p)
*/
func (t *Tendermint[V, H, A]) line34(p Prevote[H, A], prevotesForHR map[A][]Prevote[H, A]) {
	vals := slices.Collect(maps.Keys(prevotesForHR))
	if !t.state.timeoutPrevoteScheduled && t.state.s == prevote &&
		t.validatorSetVotingPower(vals) >= q(t.validators.TotalVotingPower(p.H)) {
		t.scheduleTimeout(t.timeoutPrevote(p.R), prevote, p.H, p.R)
		t.state.timeoutPrevoteScheduled = true
	}
}

/*
Check the upon condition on line 44:

	44: upon 2f + 1 {PREVOTE, h_p, round_p, nil} while step_p = prevote do
	45: broadcast {PRECOMMIT, hp, roundp, nil}
	46: step_p ← precommit

Line 36 and 44 for a round are mutually exclusive.
*/
func (t *Tendermint[V, H, A]) line44(p Prevote[H, A], prevotesForHR map[A][]Prevote[H, A]) {
	var vals []A
	for addr, valPrevotes := range prevotesForHR {
		for _, v := range valPrevotes {
			if v.ID == nil {
				vals = append(vals, addr)
			}
		}
	}

	if t.state.s == prevote && t.validatorSetVotingPower(vals) >= q(t.validators.TotalVotingPower(p.H)) {
		vote := Precommit[H, A]{
			H:      t.state.h,
			R:      t.state.r,
			ID:     nil,
			Sender: t.nodeAddr,
		}

		t.messages.addPrecommit(vote)
		t.broadcasters.PrecommitBroadcaster.Broadcast(vote)
		t.state.s = precommit
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
func (t *Tendermint[V, H, A]) line36WhenPrevoteIsReceived(p Prevote[H, A], proposalsForHR map[A][]Proposal[V, H, A],
	prevotesForHR map[A][]Prevote[H, A],
) {
	if !t.state.lockedValueAndOrValidValueSet && t.state.s >= prevote {
		proposal := t.checkForMatchingProposalGivenPrevote(p, proposalsForHR)
		vals := checkForQuorumPrevotesGivenPrevote(p, prevotesForHR)

		if proposal != nil && t.validatorSetVotingPower(vals) >= q(t.validators.TotalVotingPower(p.H)) {
			cr := t.state.r

			if t.state.s == prevote {
				t.state.lockedValue = proposal.Value
				t.state.lockedRound = int(cr)

				vote := Precommit[H, A]{
					H:      t.state.h,
					R:      t.state.r,
					ID:     p.ID,
					Sender: t.nodeAddr,
				}

				t.messages.addPrecommit(vote)
				t.broadcasters.PrecommitBroadcaster.Broadcast(vote)
				t.state.s = precommit
			}

			t.state.validValue = proposal.Value
			t.state.validRound = int(cr)
			t.state.lockedValueAndOrValidValueSet = true
		}
	}
}
