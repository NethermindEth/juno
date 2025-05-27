package tendermint

import (
	"maps"
	"slices"

	"github.com/NethermindEth/juno/consensus/types"
)

/*
Check the upon condition on line 47:

	47: upon 2f + 1 {PRECOMMIT, h_p, round_p, ∗} for the first time do
	48: schedule OnTimeoutPrecommit(h_p , round_p) to be executed after timeoutPrecommit(round_p)
*/
func (t *stateMachine[V]) uponPrecommitAny() bool {
	precommits := t.messages.Precommits[t.state.height][t.state.round]
	vals := slices.Collect(maps.Keys(precommits))

	isFirstTime := !t.state.timeoutPrecommitScheduled

	hasQuorum := t.validatorSetVotingPower(vals) >= q(t.validators.TotalVotingPower(t.state.height))

	return hasQuorum && isFirstTime
}

func (t *stateMachine[V]) doPrecommitAny() types.Action[V] {
	t.state.timeoutPrecommitScheduled = true
	return t.scheduleTimeout(types.StepPrecommit)
}
