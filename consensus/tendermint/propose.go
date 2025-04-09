package tendermint

import "github.com/NethermindEth/juno/utils"

func (t *Tendermint[V, H, A]) handleProposal(p Proposal[V, H, A]) {
	if p.H < t.state.h {
		return
	}

	if !handleFutureHeightMessage(
		t,
		p,
		func(p Proposal[V, H, A]) height { return p.H },
		func(p Proposal[V, H, A]) round { return p.R },
		t.futureMessages.addProposal,
	) {
		return
	}

	if !handleFutureRoundMessage(t, p, func(p Proposal[V, H, A]) round { return p.R }, t.futureMessages.addProposal) {
		return
	}

	if p.Sender != t.validators.Proposer(p.H, p.R) {
		return
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

	if t.uponCommitValue(cachedProposal) {
		t.doCommitValue(cachedProposal)
		return
	}

	if p.R < t.state.r {
		// Except line 49 all other upon condition which refer to the proposals expect to be acted upon
		// when the current round is equal to the proposal's round.
		return
	}

	if t.uponFirstProposal(cachedProposal) {
		t.doFirstProposal(cachedProposal)
	}

	if t.uponProposalAndPolkaPrevious(cachedProposal, p.ValidRound) {
		t.doProposalAndPolkaPrevious(cachedProposal)
	}

	if t.uponProposalAndPolkaCurrent(cachedProposal) {
		t.doProposalAndPolkaCurrent(cachedProposal)
	}
}
