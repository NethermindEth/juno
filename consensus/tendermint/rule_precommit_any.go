package tendermint

import (
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/votecounter"
)

/*
Check the upon condition on line 47:

	47: upon 2f + 1 {PRECOMMIT, h_p, round_p, ∗} for the first time do
	48: schedule OnTimeoutPrecommit(h_p , round_p) to be executed after timeoutPrecommit(round_p)
*/
func (t *stateMachine[V, H, A]) uponPrecommitAny() bool {
	isFirstTime := !t.state.timeoutPrecommitScheduled

	hasQuorum := t.voteCounter.HasQuorumForAny(t.state.round, votecounter.Precommit)

	return hasQuorum && isFirstTime
}

func (t *stateMachine[V, H, A]) doPrecommitAny() types.Action[V, H, A] {
	t.state.timeoutPrecommitScheduled = true
	return t.scheduleTimeout(types.StepPrecommit)
}
