package tendermint

func (t *Tendermint[V, H, A]) handlePrevote(p Prevote[H, A]) {
	if p.Height < t.state.height {
		return
	}

	if !handleFutureHeightMessage(
		t,
		p,
		func(p Prevote[H, A]) height { return p.Height },
		func(p Prevote[H, A]) round { return p.Round },
		t.futureMessages.addPrevote,
	) {
		return
	}

	if !handleFutureRoundMessage(t, p, func(p Prevote[H, A]) round { return p.Round }, t.futureMessages.addPrevote) {
		return
	}

	t.messages.addPrevote(p)

	cachedProposal := t.findMatchingProposal(t.state.round, p.ID)

	if cachedProposal != nil && t.uponProposalAndPolkaPrevious(cachedProposal, p.Round) {
		t.doProposalAndPolkaPrevious(cachedProposal)
	}

	if p.Round == t.state.round {
		if t.uponPolkaAny() {
			t.doPolkaAny()
		}

		if t.uponPolkaNil() {
			t.doPolkaNil()
		}
	}

	if cachedProposal != nil && t.uponProposalAndPolkaCurrent(cachedProposal) {
		t.doProposalAndPolkaCurrent(cachedProposal)
	}
}
