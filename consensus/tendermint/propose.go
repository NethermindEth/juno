package tendermint

import "github.com/NethermindEth/juno/utils"

func (t *Tendermint[V, H, A]) handleProposal(p Proposal[V, H, A]) Action[V, H, A] {
	if action, ok := t.preprocessMessage(p.MessageHeader, func() { t.messages.addProposal(p) }); !ok {
		return action
	}

	if p.Sender != t.validators.Proposer(p.Height, p.Round) {
		return nil
	}

	// The code below shouldn't panic because it is expected Proposal is well-formed. However, there need to be a way to
	// distinguish between nil and zero value. This is expected to be handled by the p2p layer.
	cachedProposal := &CachedProposal[V, H, A]{
		Proposal: p,
		Valid:    t.application.Valid(*p.Value),
		ID:       utils.HeapPtr((*p.Value).Hash()),
	}

	if cachedProposal.Valid {
		// Add the proposal to the message set even if the sender is not the proposer,
		// this is because of slahsing purposes
		t.messages.addProposal(p)
	}

	if t.uponProposalAndPrecommitValue(cachedProposal) {
		return t.doProposalAndPrecommitValue(cachedProposal)
	}

	if p.Round < t.state.round {
		// Except line 49 all other upon condition which refer to the proposals expect to be acted upon
		// when the current round is equal to the proposal's round.
		return nil
	}

	if t.uponFirstProposal(cachedProposal) {
		return t.doFirstProposal(cachedProposal)
	}

	if t.uponProposalAndPolkaPrevious(cachedProposal, p.ValidRound) {
		return t.doProposalAndPolkaPrevious(cachedProposal)
	}

	if t.uponProposalAndPolkaCurrent(cachedProposal) {
		return t.doProposalAndPolkaCurrent(cachedProposal)
	}

	return nil
}
