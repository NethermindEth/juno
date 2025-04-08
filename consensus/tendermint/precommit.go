package tendermint

func (t *Tendermint[V, H, A]) handlePrecommit(p Precommit[H, A]) Action[V, H, A] {
	if action, ok := t.preprocessMessage(p.MessageHeader, func() { t.messages.addPrecommit(p) }); !ok {
		return action
	}

	t.messages.addPrecommit(p)

	cachedProposal := t.findProposal(p.Round)

	if cachedProposal != nil && t.uponProposalAndPrecommitValue(cachedProposal) {
		return t.doProposalAndPrecommitValue(cachedProposal)
	}

	if t.uponPrecommitAny() {
		return t.doPrecommitAny()
	}

	return nil
}
