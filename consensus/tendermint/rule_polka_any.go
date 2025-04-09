package tendermint

import (
	"maps"
	"slices"
)

/*
Check the upon condition on line 34:

	34: upon 2f + 1 {PREVOTE, h_p, round_p, ∗} while step_p = prevote for the first time do
	35: schedule OnTimeoutPrevote(h_p, round_p) to be executed after timeoutPrevote(round_p)
*/
func (t *Tendermint[V, H, A]) uponPolkaAny(p Prevote[H, A]) bool {
	prevotes := t.messages.prevotes[t.state.h][t.state.r]
	vals := slices.Collect(maps.Keys(prevotes))

	isFirstTime := !t.state.timeoutPrevoteScheduled

	hasQuorum := t.validatorSetVotingPower(vals) >= q(t.validators.TotalVotingPower(p.H))

	return p.R == t.state.r &&
		t.state.s == prevote &&
		hasQuorum &&
		isFirstTime
}

func (t *Tendermint[V, H, A]) doPolkaAny(p Prevote[H, A]) {
	t.scheduleTimeout(t.timeoutPrevote(p.R), prevote, p.H, p.R)
	t.state.timeoutPrevoteScheduled = true
}
