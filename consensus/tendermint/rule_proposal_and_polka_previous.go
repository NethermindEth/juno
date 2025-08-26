package tendermint

import (
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/votecounter"
)

/*
Check the upon condition on line 28:

	28: upon {PROPOSAL, h_p, round_p, v, vr} from proposer(h_p, round_p) AND 2f + 1 {PREVOTE,h_p, vr, id(v)} while
		step_p = propose ∧ (vr ≥ 0 ∧ vr < round_p) do
	29: if valid(v) ∧ (lockedRound_p ≤ vr ∨ lockedValue_p = v) then
	30: 	broadcast {PREVOTE, hp, round_p, id(v)}
	31: else
	32:  	broadcast {PREVOTE, hp, round_p, nil}
	33: step_p ← prevote
*/
func (s *stateMachine[V, H, A]) uponProposalAndPolkaPrevious(
	cachedProposal *CachedProposal[V, H, A],
) bool {
	vr := cachedProposal.ValidRound
	hasQuorum := cachedProposal.ID != nil &&
		s.voteCounter.HasQuorumForVote(vr, votecounter.Prevote, cachedProposal.ID)
	return hasQuorum &&
		s.state.step == types.StepPropose &&
		vr >= 0 &&
		vr < s.state.round
}

func (s *stateMachine[V, H, A]) doProposalAndPolkaPrevious(
	cachedProposal *CachedProposal[V, H, A],
) types.Action[V, H, A] {
	var votedID *H
	shouldVoteForValue := cachedProposal.Valid &&
		(s.state.lockedRound <= cachedProposal.ValidRound ||
			s.state.lockedValue != nil &&
				cachedProposal.ID != nil &&
				(*s.state.lockedValue).Hash() == *cachedProposal.ID)

	if shouldVoteForValue {
		votedID = cachedProposal.ID
	}
	return s.setStepAndSendPrevote(votedID)
}
