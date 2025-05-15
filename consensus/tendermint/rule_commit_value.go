package tendermint

import "github.com/NethermindEth/juno/consensus/types"

/*
Check the upon condition on line 49:

	49: upon {PROPOSAL, h_p, r, v, *} from proposer(h_p, r) AND 2f + 1 {PRECOMMIT, h_p, r, id(v)} while decision_p[h_p] = nil do
	50: 	if valid(v) then
	51: 		decisionp[hp] = v
	52: 		h_p ← h_p + 1
	53: 		reset lockedRound_p, lockedValue_p,	validRound_p and validValue_p to initial values and empty message log
	54: 		StartRound(0)

Fetching the relevant proposal implies the sender of the proposal was the proposer for that
height and round.

There is no need to check decision_p[h_p] = nil since it is implied that decision are made
sequentially, i.e. x, x+1, x+2... .
*/
func (t *stateMachine[V, H, A]) uponCommitValue(cachedProposal *CachedProposal[V, H, A]) bool {
	_, hasQuorum := t.checkForQuorumPrecommit(cachedProposal.Round, *cachedProposal.ID)

	// This is checked here instead of inside execution, because it's the only case in execution in this rule
	isValid := cachedProposal.Valid

	// h_p never goes backward, so it's safe to assume that decision_p[h_p] is nil
	return hasQuorum && isValid
}

func (t *stateMachine[V, H, A]) doCommitValue(cachedProposal *CachedProposal[V, H, A]) types.Action[V, H, A] {
	if err := t.db.Flush(); err != nil {
		t.log.Fatalf("failed to flush WAL during commit", "height", cachedProposal.Height, "round", cachedProposal.Round, "err", err)
	}

	// TODO: Optimise this
	precommits, _ := t.checkForQuorumPrecommit(cachedProposal.Round, *cachedProposal.ID)
	t.blockchain.Commit(t.state.height, *cachedProposal.Value, precommits)

	if err := t.db.DeleteWALEntries(t.state.height); err != nil {
		t.log.Errorw("failed to delete WAL messages during commit", "height", cachedProposal.Height, "round", cachedProposal.Round, "err", err)
	}

	t.messages.DeleteHeightMessages(t.state.height)
	t.state.height++
	return t.startRound(0)
}
