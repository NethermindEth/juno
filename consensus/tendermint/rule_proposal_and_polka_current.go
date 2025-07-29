package tendermint

import (
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/votecounter"
)

/*
Check upon condition on line 36:

	36: upon {PROPOSAL, h_p, round_p, v, ∗} from proposer(h_p, round_p) AND 2f + 1 {PREVOTE, h_p, round_p, id(v)} while
		valid(v) ∧ step_p ≥ prevote for the first time do
	37: if step_p = prevote then
	38: 	lockedValue_p ← v
	39: 	lockedRound_p ← round_p
	40: 	broadcast {PRECOMMIT, h_p, round_p, id(v))}
	41: 	step_p ← precommit
	42: validValue_p ← v
	43: validRound_p ← round_p
*/
func (s *stateMachine[V, H, A]) uponProposalAndPolkaCurrent(cachedProposal *CachedProposal[V, H, A]) bool {
	hasQuorum := cachedProposal.ID != nil && s.voteCounter.HasQuorumForVote(s.state.round, votecounter.Prevote, cachedProposal.ID)
	firstTime := !s.state.lockedValueAndOrValidValueSet
	return hasQuorum &&
		cachedProposal.Valid &&
		s.state.step >= types.StepPrevote &&
		firstTime
}

func (s *stateMachine[V, H, A]) doProposalAndPolkaCurrent(cachedProposal *CachedProposal[V, H, A]) types.Action[V, H, A] {
	var action types.Action[V, H, A]
	if s.state.step == types.StepPrevote {
		s.state.lockedValue = cachedProposal.Value
		s.state.lockedRound = s.state.round
		action = s.setStepAndSendPrecommit(cachedProposal.ID)
	}

	s.state.validValue = cachedProposal.Value
	s.state.validRound = s.state.round
	s.state.lockedValueAndOrValidValueSet = true

	return action
}
