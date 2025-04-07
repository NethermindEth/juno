package tendermint

func (t *Tendermint[V, H, A]) processExistingMessages() {
	cachedProposal := t.findProposal(t.state.round)
	if cachedProposal != nil {
		if t.uponProposalAndPrecommitValue(cachedProposal) {
			t.doProposalAndPrecommitValue(cachedProposal)
			return
		}

		if t.uponFirstProposal(cachedProposal) {
			t.doFirstProposal(cachedProposal)
		}

		if t.uponProposalAndPolkaCurrent(cachedProposal) {
			t.doProposalAndPolkaCurrent(cachedProposal)
		}
	}

	if t.uponPolkaAny() {
		t.doPolkaAny()
	}

	if t.uponPolkaNil() {
		t.doPolkaNil()
	}

	if t.uponPrecommitAny() {
		t.doPrecommitAny()
	}
}
