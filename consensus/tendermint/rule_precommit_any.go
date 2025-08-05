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
func (s *stateMachine[V, H, A]) uponPrecommitAny() bool {
	isFirstTime := !s.state.timeoutPrecommitScheduled

	hasQuorum := s.voteCounter.HasQuorumForAny(s.state.round, votecounter.Precommit)
	return hasQuorum && isFirstTime
}

func (s *stateMachine[V, H, A]) doPrecommitAny() types.Action[V, H, A] {
	s.state.timeoutPrecommitScheduled = true
	return s.scheduleTimeout(types.StepPrecommit)
}
