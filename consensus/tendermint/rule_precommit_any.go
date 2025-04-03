package tendermint

import (
	"maps"
	"slices"
)

/*
Check the upon condition on line 47:

	47: upon 2f + 1 {PRECOMMIT, h_p, round_p, ∗} for the first time do
	48: schedule OnTimeoutPrecommit(h_p , round_p) to be executed after timeoutPrecommit(round_p)
*/
func (t *Tendermint[V, H, A]) line47(p Precommit[H, A], precommitsForHR map[A][]Precommit[H, A]) {
	vals := slices.Collect(maps.Keys(precommitsForHR))
	if p.R == t.state.r && !t.state.timeoutPrecommitScheduled &&
		t.validatorSetVotingPower(vals) >= q(t.validators.TotalVotingPower(p.H)) {
		t.scheduleTimeout(t.timeoutPrecommit(p.R), precommit, p.H, p.R)
		t.state.timeoutPrecommitScheduled = true
	}
}
