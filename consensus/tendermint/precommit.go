package tendermint

import (
	"fmt"
	"maps"
	"slices"
)

func (t *Tendermint[V, H, A]) handlePrecommit(p Precommit[H, A]) {
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
		t.futureMessages.addPrecommit(p)
		return
	}

	if p.Round > t.state.round {
		if p.Round-t.state.round > maxFutureRound {
			return
		}

		t.futureMessagesMu.Lock()
		defer t.futureMessagesMu.Unlock()

		t.futureMessages.addPrecommit(p)

		/*
			Check upon condition line 55:

				55: upon f + 1 {∗, h_p, round, ∗, ∗} with round > round_p do
				56: 	StartRound(round)
		*/

		t.line55(p.Round)
		return
	}

	fmt.Println("got precommmit")
	t.messages.addPrecommit(p)

	proposalsForHR, _, precommitsForHR := t.messages.allMessages(p.Height, p.Round)

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

	if p.ID != nil {
		var (
			proposal   *Proposal[V, H, A]
			precommits []Precommit[H, A]
			vals       []A
		)

		for _, prop := range proposalsForHR[t.validators.Proposer(p.Height, p.Round)] {
			if (*prop.Value).Hash() == *p.ID {
				propCopy := prop
				proposal = &propCopy
			}
		}

		for addr, valPrecommits := range precommitsForHR {
			for _, v := range valPrecommits {
				if *v.ID == *p.ID {
					precommits = append(precommits, v)
					vals = append(vals, addr)
				}
			}
		}
		if proposal != nil && t.validatorSetVotingPower(vals) >= q(t.validators.TotalVotingPower(p.Height)) {
			t.blockchain.Commit(t.state.height, *proposal.Value, precommits)

			t.messages.deleteHeightMessages(t.state.height)
			t.state.height++
			t.startRound(0)

			return
		}
	}

	/*
		Check the upon condition on line 47:

			47: upon 2f + 1 {PRECOMMIT, h_p, round_p, ∗} for the first time do
			48: schedule OnTimeoutPrecommit(h_p , round_p) to be executed after timeoutPrecommit(round_p)
	*/

	vals := slices.Collect(maps.Keys(precommitsForHR))
	if p.Round == t.state.round && !t.state.line47Executed &&
		t.validatorSetVotingPower(vals) >= q(t.validators.TotalVotingPower(p.Height)) {
		t.scheduleTimeout(t.timeoutPrecommit(p.Round), precommit, p.Height, p.Round)
		t.state.line47Executed = true
	}
}
