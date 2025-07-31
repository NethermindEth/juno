package tendermint

import (
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/votecounter"
)

/*
Check the upon condition on line 44:

	44: upon 2f + 1 {PREVOTE, h_p, round_p, nil} while step_p = prevote do
	45: broadcast {PRECOMMIT, hp, roundp, nil}
	46: step_p ← precommit

Line 36 and 44 for a round are mutually exclusive.
*/
func (s *stateMachine[V, H, A]) uponPolkaNil() bool {
	hasQuorum := s.voteCounter.HasQuorumForVote(s.state.round, votecounter.Prevote, nil)

	return hasQuorum && s.state.step == types.StepPrevote
}

func (s *stateMachine[V, H, A]) doPolkaNil() types.Action[V, H, A] {
	return s.setStepAndSendPrecommit(nil)
}
