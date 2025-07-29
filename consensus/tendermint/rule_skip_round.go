package tendermint

import (
	"github.com/NethermindEth/juno/consensus/types"
)

/*
Check the upon condition on line 55:

	55: upon f + 1 {∗, h_p, round, ∗, ∗} with round > round_p do
	56: 	StartRound(round)

If there are f + 1 messages from a newer round, there is at least an honest node in that round.
*/
func (s *stateMachine[V, H, A]) uponSkipRound(futureR types.Round) bool {
	isNewerRound := futureR > s.state.round

	hasQuorum := s.voteCounter.HasNonFaultyFutureMessage(futureR)

	return isNewerRound && hasQuorum
}

func (s *stateMachine[V, H, A]) doSkipRound(futureR types.Round) types.Action[V, H, A] {
	return s.startRound(futureR)
}
