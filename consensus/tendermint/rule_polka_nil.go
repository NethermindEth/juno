package tendermint

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
		t.sendPrecommit(nil)
	}
}
