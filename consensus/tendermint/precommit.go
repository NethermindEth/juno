package tendermint

func (t *Tendermint[V, H, A]) handlePrecommit(p Precommit[H, A]) {
	if !t.preprocessMessage(p.MessageHeader, func() { t.messages.addPrecommit(p) }) {
		return
	}

	t.messages.addPrecommit(p)

	cachedProposal := t.findProposal(p.Round)

	if cachedProposal != nil && t.uponProposalAndPrecommitValue(cachedProposal) {
		t.doProposalAndPrecommitValue(cachedProposal)
		return
	}

	if t.uponPrecommitAny() {
		t.doPrecommitAny()
	}
}
