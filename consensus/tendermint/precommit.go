package tendermint

func (t *Tendermint[V, H, A]) handlePrecommit(p Precommit[H, A]) {
	if p.H < t.state.h {
		return
	}

	if !handleFutureHeightMessage(
		t,
		p,
		func(p Precommit[H, A]) height { return p.H },
		func(p Precommit[H, A]) round { return p.R },
		t.futureMessages.addPrecommit,
	) {
		return
	}

	if !handleFutureRoundMessage(t, p, func(p Precommit[H, A]) round { return p.R }, t.futureMessages.addPrecommit) {
		return
	}

	t.messages.addPrecommit(p)

	_, _, precommitsForHR := t.messages.allMessages(p.H, p.R)

	if t.line49WhenPrecommitIsReceived(p) {
		return
	}

	t.line47(p, precommitsForHR)
}
