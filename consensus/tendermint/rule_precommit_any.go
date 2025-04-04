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
func (t *Tendermint[V, H, A]) uponPrecommitAny() bool {
	precommits := t.messages.precommits[t.state.height][t.state.round]
	vals := slices.Collect(maps.Keys(precommits))

	isFirstTime := !t.state.timeoutPrecommitScheduled

	hasQuorum := t.validatorSetVotingPower(vals) >= q(t.validators.TotalVotingPower(t.state.height))

	return hasQuorum && isFirstTime
}

func (t *Tendermint[V, H, A]) doPrecommitAny() {
	t.scheduleTimeout(t.timeoutPrecommit(t.state.round), precommit, t.state.height, t.state.round)
	t.state.timeoutPrecommitScheduled = true
}
