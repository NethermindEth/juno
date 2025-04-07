package tendermint

import (
	"maps"
	"slices"
)

// line55 assumes the caller has acquired a mutex for accessing future messages.
/*
	55: upon f + 1 {∗, h_p, round, ∗, ∗} with round > round_p do
	56: 	StartRound(round)
*/
func (t *Tendermint[V, H, A]) line55(futureR round) {
	t.futureMessagesMu.Lock()

	vals := make(map[A]struct{})
	proposals, prevotes, precommits := t.futureMessages.allMessages(t.state.h, futureR)

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

	t.futureMessagesMu.Unlock()

	if t.validatorSetVotingPower(slices.Collect(maps.Keys(vals))) > f(t.validators.TotalVotingPower(t.state.h)) {
		t.startRound(futureR)
	}
}
