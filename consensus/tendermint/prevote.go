package tendermint

func (t *Tendermint[V, H, A]) handlePrevote(p Prevote[H, A]) {
	if !t.preprocessMessage(p.MessageHeader, func() { t.futureMessages.addPrevote(p) }) {
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
