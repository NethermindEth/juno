package tendermint

func (t *Tendermint[V, H, A]) handlePrecommit(p Precommit[H, A]) {
	if p.Height < t.state.height {
		return
	}

	if !handleFutureHeightMessage(
		t,
		p,
		func(p Precommit[H, A]) height { return p.Height },
		func(p Precommit[H, A]) round { return p.Round },
		t.futureMessages.addPrecommit,
	) {
		return
	}

	if !handleFutureRoundMessage(t, p, func(p Precommit[H, A]) round { return p.Round }, t.futureMessages.addPrecommit) {
		return
	}

	t.messages.addPrecommit(p)

	cachedProposal := t.findProposal(p.Round)

	if cachedProposal != nil && t.uponCommitValue(cachedProposal) {
		t.doCommitValue(cachedProposal)
		return
	}

	if t.uponPrecommitAny(p) {
		t.doPrecommitAny(p)
	}
}
