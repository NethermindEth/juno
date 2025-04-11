package tendermint

import (
	"maps"
	"slices"
)

/*
Check the upon condition on line 55:

	55: upon f + 1 {∗, h_p, round, ∗, ∗} with round > round_p do
	56: 	StartRound(round)

If there are f + 1 messages from a newer round, there is at least an honest node in that round.
*/
func (t *Tendermint[V, H, A]) uponSkipRound(futureR round) bool {
	vals := make(map[A]struct{})
	proposals, prevotes, precommits := t.messages.allMessages(t.state.height, futureR)

	// If a validator has sent proposl, prevote and precommit from a future round then it will only be counted once.
	for addr := range proposals {
		vals[addr] = struct{}{}
	}

	for addr := range prevotes {
		vals[addr] = struct{}{}
	}

	for addr := range precommits {
		vals[addr] = struct{}{}
	}

	isNewerRound := futureR > t.state.round

	hasQuorum := t.validatorSetVotingPower(slices.Collect(maps.Keys(vals))) > f(t.validators.TotalVotingPower(t.state.height))

	return isNewerRound && hasQuorum
}

func (t *Tendermint[V, H, A]) doSkipRound(futureR round) Action[V, H, A] {
	return t.startRound(futureR)
}
