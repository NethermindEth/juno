package tendermint

import (
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/votecounter"
)

/*
Check the upon condition on line 34:

	34: upon 2f + 1 {PREVOTE, h_p, round_p, ∗} while step_p = prevote for the first time do
	35: schedule OnTimeoutPrevote(h_p, round_p) to be executed after timeoutPrevote(round_p)
*/
func (t *stateMachine[V, H, A]) uponPolkaAny() bool {
	isFirstTime := !t.state.timeoutPrevoteScheduled

	hasQuorum := t.voteCounter.HasQuorumForAny(t.state.round, votecounter.Prevote)

	return t.state.step == types.StepPrevote &&
		hasQuorum &&
		isFirstTime
}

func (t *stateMachine[V, H, A]) doPolkaAny() types.Action[V, H, A] {
	t.state.timeoutPrevoteScheduled = true
	return t.scheduleTimeout(types.StepPrevote)
}
