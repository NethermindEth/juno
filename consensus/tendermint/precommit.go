package tendermint

func (t *Tendermint[V, H, A]) handlePrecommit(p Precommit[H, A]) {
	if !t.preprocessMessage(p.MessageHeader, func() { t.futureMessages.addPrecommit(p) }) {
		return
	}

	t.messages.addPrecommit(p)

	cachedProposal := t.findProposal(p.Round)

	if cachedProposal != nil && t.uponCommitValue(cachedProposal) {
		t.doCommitValue(cachedProposal)
		return
	}

	if t.uponPrecommitAny() {
		t.doPrecommitAny()
	}
}
