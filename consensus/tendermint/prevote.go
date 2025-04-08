package tendermint

func (t *Tendermint[V, H, A]) handlePrevote(p Prevote[H, A]) Action[V, H, A] {
	if action, ok := t.preprocessMessage(p.MessageHeader, func() { t.messages.addPrevote(p) }); !ok {
		return action
	}

	t.messages.addPrevote(p)

	cachedProposal := t.findProposal(t.state.round)

	if cachedProposal != nil && t.uponProposalAndPolkaPrevious(cachedProposal, p.Round) {
		return t.doProposalAndPolkaPrevious(cachedProposal)
	}

	if p.Round == t.state.round {
		if t.uponPolkaAny() {
			return t.doPolkaAny()
		}

		if t.uponPolkaNil() {
			return t.doPolkaNil()
		}
	}

	if cachedProposal != nil && t.uponProposalAndPolkaCurrent(cachedProposal) {
		return t.doProposalAndPolkaCurrent(cachedProposal)
	}

	return nil
}
