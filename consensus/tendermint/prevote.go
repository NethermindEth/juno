package tendermint

func (t *Tendermint[V, H, A]) handlePrevote(p Prevote[H, A]) {
	if p.H < t.state.h {
		return
	}

	if !handleFutureHeightMessage(
		t,
		p,
		func(p Prevote[H, A]) height { return p.H },
		func(p Prevote[H, A]) round { return p.R },
		t.futureMessages.addPrevote,
	) {
		return
	}

	if !handleFutureRoundMessage(t, p, func(p Prevote[H, A]) round { return p.R }, t.futureMessages.addPrevote) {
		return
	}

	t.messages.addPrevote(p)

	cachedProposal := t.findMatchingProposal(t.state.r, p.ID)

	if cachedProposal != nil && t.uponProposalAndPolkaPrevious(cachedProposal, p.R) {
		t.doProposalAndPolkaPrevious(cachedProposal)
	}

	if p.R == t.state.r {
		if t.uponPolkaAny(p) {
			t.doPolkaAny(p)
		}

		if t.uponPolkaNil(p) {
			t.doPolkaNil()
		}
	}

	if cachedProposal != nil && t.uponProposalAndPolkaCurrent(cachedProposal) {
		t.doProposalAndPolkaCurrent(cachedProposal)
	}
}
