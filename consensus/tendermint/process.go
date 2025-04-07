package tendermint

func (t *Tendermint[V, H, A]) processMessage(header MessageHeader[A], addMessage func()) {
	if !t.preprocessMessage(header, addMessage) {
		return
	}

	t.processLoop(&header.Round)
}

func (t *Tendermint[V, H, A]) processTimeout(tm timeout) {
	switch tm.s {
	case propose:
		t.onTimeoutPropose(tm.h, tm.r)
	case prevote:
		t.onTimeoutPrevote(tm.h, tm.r)
	case precommit:
		t.onTimeoutPrecommit(tm.h, tm.r)
	}
	t.processLoop(nil)
}

func (t *Tendermint[V, H, A]) processLoop(recentlyReceivedRound *round) {
	shouldContinue := t.process(recentlyReceivedRound)
	for shouldContinue {
		shouldContinue = t.process(recentlyReceivedRound)
	}
}

func (t *Tendermint[V, H, A]) process(recentlyReceivedRound *round) bool {
	cachedProposal := t.findProposal(t.state.round)

	var roundCachedProposal *CachedProposal[V, H, A]
	if recentlyReceivedRound != nil {
		roundCachedProposal = t.findProposal(*recentlyReceivedRound)
	}

	switch {
	// Line 22
	case cachedProposal != nil && t.uponFirstProposal(cachedProposal):
		t.doFirstProposal(cachedProposal)

	// Line 28
	case cachedProposal != nil && t.uponProposalAndPolkaPrevious(cachedProposal):
		t.doProposalAndPolkaPrevious(cachedProposal)

	// Line 34
	case t.uponPolkaAny():
		t.doPolkaAny()

	// Line 36
	case cachedProposal != nil && t.uponProposalAndPolkaCurrent(cachedProposal):
		t.doProposalAndPolkaCurrent(cachedProposal)

	// Line 44
	case t.uponPolkaNil():
		t.doPolkaNil()

	// Line 47
	case t.uponPrecommitAny():
		t.doPrecommitAny()

	// Line 49
	case roundCachedProposal != nil && t.uponCommitValue(roundCachedProposal):
		t.doCommitValue(roundCachedProposal)

	// Line 55
	case recentlyReceivedRound != nil && t.uponSkipRound(*recentlyReceivedRound):
		t.doSkipRound(*recentlyReceivedRound)

	default:
		return false
	}

	return true
}
