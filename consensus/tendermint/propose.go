package tendermint

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

	// The code below shouldn't panic because it is expected Proposal is well-formed. However, there need to be a way to
	// distinguish between nil and zero value. This is expected to be handled by the p2p layer.
	vID := (*p.Value).Hash()
	validProposal := t.application.Valid(*p.Value)
	proposalFromProposer := p.Sender == t.validators.Proposer(p.H, p.R)
	vr := p.ValidRound

	if validProposal {
		// Add the proposal to the message set even if the sender is not the proposer,
		// this is because of slahsing purposes
		t.messages.addProposal(p)
	}

	_, prevotesForHR, _ := t.messages.allMessages(p.H, p.R)

	if t.line49WhenProposalIsReceived(p, vID, validProposal, proposalFromProposer) {
		return
	}

	if p.R < t.state.r {
		// Except line 49 all other upon condition which refer to the proposals expect to be acted upon
		// when the current round is equal to the proposal's round.
		return
	}

	t.line22(vr, proposalFromProposer, validProposal, vID)
	t.line28WhenProposalIsReceived(vr, proposalFromProposer, vID, validProposal)
	t.line36WhenProposalIsReceived(p, validProposal, proposalFromProposer, prevotesForHR, vID)
}
