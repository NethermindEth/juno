package tendermint

/*
Check the upon condition on line 44:

	44: upon 2f + 1 {PREVOTE, h_p, round_p, nil} while step_p = prevote do
	45: broadcast {PRECOMMIT, hp, roundp, nil}
	46: step_p ← precommit

Line 36 and 44 for a round are mutually exclusive.
*/
func (t *Tendermint[V, H, A]) uponPolkaNil(p Prevote[H, A]) bool {
	prevotes := t.messages.prevotes[t.state.h][t.state.r]

	var vals []A
	for addr, valPrevotes := range prevotes {
		for _, v := range valPrevotes {
			if v.ID == nil {
				vals = append(vals, addr)
			}
		}
	}

	// TODO: refactor this
	hasQuorum := t.validatorSetVotingPower(vals) >= q(t.validators.TotalVotingPower(p.H))

	return hasQuorum && t.state.s == prevote
}

func (t *Tendermint[V, H, A]) doPolkaNil() {
	t.sendPrecommit(nil)
}
