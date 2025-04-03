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

	_, prevotesForHR, _ := t.messages.allMessages(p.H, p.R)

	t.line28WhenPrevoteIsReceived(p)

	if p.R == t.state.r {
		t.line34(p, prevotesForHR)
		t.line44(p, prevotesForHR)

		t.line36WhenPrevoteIsReceived(p)
	}
}
