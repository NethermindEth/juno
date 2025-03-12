package tendermint

import (
	"fmt"
)

//nolint:funlen
func (t *Tendermint[V, H, A]) handleProposal(p Proposal[V, H, A]) {
	if p.Height < t.state.height {
		return
	}

	if p.Height > t.state.height {
		if p.Height-t.state.height > maxFutureHeight {
			return
		}

		if p.Round > maxFutureRound {
			return
		}

		t.futureMessagesMu.Lock()
		defer t.futureMessagesMu.Unlock()
		t.futureMessages.addProposal(p)
		return
	}

	if p.Round > t.state.round {
		if p.Round-t.state.round > maxFutureRound {
			return
		}

		t.futureMessagesMu.Lock()
		defer t.futureMessagesMu.Unlock()

		t.futureMessages.addProposal(p)

		/*
			Check upon condition line 55:

				55: upon f + 1 {∗, h_p, round, ∗, ∗} with round > round_p do
				56: 	StartRound(round)
		*/

		t.line55(p.Round)
		return
	}

	// The code below shouldn't panic because it is expected Proposal is well-formed. However, there need to be a way to
	// distinguish between nil and zero value. This is expected to be handled by the p2p layer.
	vID := (*p.Value).Hash()
	fmt.Println("got proposal with valueID and validRound", vID, p.ValidRound)
	validProposal := t.application.Valid(*p.Value)
	proposalFromProposer := p.Sender == t.validators.Proposer(p.Height, p.Round)
	vr := p.ValidRound

	if validProposal {
		fmt.Println("proposal is valid. Adding to messages")
		// Add the proposal to the message set even if the sender is not the proposer,
		// this is because of slahsing purposes
		t.messages.addProposal(p)
	}

	_, prevotesForHR, precommitsForHR := t.messages.allMessages(p.Height, p.Round)

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

	var precommits []Precommit[H, A]
	var vals []A

	for addr, valPrecommits := range precommitsForHR {
		for _, p := range valPrecommits {
			if *p.ID == vID {
				precommits = append(precommits, p)
				vals = append(vals, addr)
			}
		}
	}

	if validProposal && proposalFromProposer &&
		t.validatorSetVotingPower(vals) >= q(t.validators.TotalVotingPower(p.Height)) {
		// After committing the block, how the new height and round is started needs to be coordinated
		// with the synchronisation process.
		t.blockchain.Commit(t.state.height, *p.Value, precommits)

		t.messages.deleteHeightMessages(t.state.height)
		t.state.height++
		t.startRound(0)

		return
	}

	if p.Round < t.state.round {
		// Except line 49 all other upon condition which refer to the proposals expect to be acted upon
		// when the current round is equal to the proposal's round.
		return
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

	if vr == nil && proposalFromProposer && t.state.step == propose {
		vote := Prevote[H, A]{
			Vote: Vote[H, A]{
				Height: t.state.height,
				Round:  t.state.round,
				ID:     nil,
				Sender: t.nodeAddr,
			},
		}

		if validProposal && (t.state.lockedRound == nil || (*t.state.lockedValue).Hash() == vID) {
			vote.ID = &vID
		}

		t.messages.addPrevote(vote)
		fmt.Println("About to broadcase prevote line 22")
		t.broadcasters.PrevoteBroadcaster.Broadcast(vote)
		t.state.step = prevote
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

		Ideally the condition on line 28 would be checked in a single if statement, however,
		this cannot be done because valid round needs to be non-nil before the prevotes are fetched.
	*/

	if vr != nil && proposalFromProposer && t.state.step == propose && *vr >= uint(0) && *vr < t.state.round {
		_, prevotesForHVr, _ := t.messages.allMessages(p.Height, *vr)

		vals = []A{}
		for addr, valPrevotes := range prevotesForHVr {
			for _, p := range valPrevotes {
				if *p.ID == vID {
					vals = append(vals, addr)
				}
			}
		}

		if t.validatorSetVotingPower(vals) >= q(t.validators.TotalVotingPower(p.Height)) {
			vote := Prevote[H, A]{
				Vote: Vote[H, A]{
					Height: t.state.height,
					Round:  t.state.round,
					ID:     nil,
					Sender: t.nodeAddr,
				},
			}

			if validProposal && (*t.state.lockedRound >= *vr || (*t.state.lockedValue).Hash() == vID) {
				vote.ID = &vID
			}

			t.messages.addPrevote(vote)
			fmt.Println("About to broadcase prevote line 28")
			t.broadcasters.PrevoteBroadcaster.Broadcast(vote)
			t.state.step = prevote
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
		checking for quroum is more resource intensive than other conditions therefore they are checked
		first.
	*/

	if validProposal && proposalFromProposer && !t.state.line36Executed && t.state.step >= prevote {
		vals = []A{}
		for addr, valPrevotes := range prevotesForHR {
			for _, v := range valPrevotes {
				if *v.ID == vID {
					vals = append(vals, addr)
				}
			}
		}

		if t.validatorSetVotingPower(vals) >= q(t.validators.TotalVotingPower(p.Height)) {
			cr := t.state.round

			if t.state.step == prevote {
				t.state.lockedValue = p.Value
				t.state.lockedRound = &cr

				vote := Precommit[H, A]{
					Vote: Vote[H, A]{
						Height: t.state.height,
						Round:  t.state.round,
						ID:     &vID,
						Sender: t.nodeAddr,
					},
				}

				t.messages.addPrecommit(vote)
				fmt.Println("About to broadcase prevote line 36")
				t.broadcasters.PrecommitBroadcaster.Broadcast(vote)
				t.state.step = precommit
			}

			t.state.validValue = p.Value
			t.state.validRound = &cr
			t.state.line36Executed = true
		}
	}
}
