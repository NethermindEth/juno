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
func (t *Tendermint[V, H, A]) line34(p Prevote[H, A], prevotesForHR map[A][]Prevote[H, A]) {
	vals := slices.Collect(maps.Keys(prevotesForHR))
	if !t.state.timeoutPrevoteScheduled && t.state.s == prevote &&
		t.validatorSetVotingPower(vals) >= q(t.validators.TotalVotingPower(p.H)) {
		t.scheduleTimeout(t.timeoutPrevote(p.R), prevote, p.H, p.R)
		t.state.timeoutPrevoteScheduled = true
	}
}
