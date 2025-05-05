package tendermint

func (t *Tendermint[V, H, A]) processStart(round round) []Action[V, H, A] {
	return t.processLoop(t.startRound(round), nil)
}

func (t *Tendermint[V, H, A]) processProposal(p Proposal[V, H, A]) []Action[V, H, A] {
	return t.processMessage(p.MessageHeader, func() { t.messages.addProposal(p) })
}

func (t *Tendermint[V, H, A]) processPrevote(p Prevote[H, A]) []Action[V, H, A] {
	return t.processMessage(p.MessageHeader, func() { t.messages.addPrevote(p) })
}

func (t *Tendermint[V, H, A]) processPrecommit(p Precommit[H, A]) []Action[V, H, A] {
	return t.processMessage(p.MessageHeader, func() { t.messages.addPrecommit(p) })
}

func (t *Tendermint[V, H, A]) processMessage(header MessageHeader[A], addMessage func()) []Action[V, H, A] {
	if !t.preprocessMessage(header, addMessage) {
		return nil
	}

	return t.processLoop(nil, &header.Round)
}

func (t *Tendermint[V, H, A]) processTimeout(tm timeout) []Action[V, H, A] {
	switch tm.s {
	case propose:
		return t.processLoop(t.onTimeoutPropose(tm.h, tm.r), nil)
	case prevote:
		return t.processLoop(t.onTimeoutPrevote(tm.h, tm.r), nil)
	case precommit:
		return t.processLoop(t.onTimeoutPrecommit(tm.h, tm.r), nil)
	}

	return nil
}

func (t *Tendermint[V, H, A]) processLoop(action Action[V, H, A], recentlyReceivedRound *round) []Action[V, H, A] {
	actions := []Action[V, H, A]{}
	if action != nil {
		actions = append(actions, action)
	}

	action = t.process(recentlyReceivedRound)
	for action != nil {
		actions = append(actions, action)
		action = t.process(recentlyReceivedRound)
	}

	return actions
}

func (t *Tendermint[V, H, A]) process(recentlyReceivedRound *round) Action[V, H, A] {
	cachedProposal := t.findProposal(t.state.round)

	var roundCachedProposal *CachedProposal[V, H, A]
	if recentlyReceivedRound != nil {
		roundCachedProposal = t.findProposal(*recentlyReceivedRound)
	}

	switch {
	// Line 22
	case cachedProposal != nil && t.uponFirstProposal(cachedProposal):
		return t.doFirstProposal(cachedProposal)

	// Line 28
	case cachedProposal != nil && t.uponProposalAndPolkaPrevious(cachedProposal):
		return t.doProposalAndPolkaPrevious(cachedProposal)

	// Line 34
	case t.uponPolkaAny():
		return t.doPolkaAny()

	// Line 36
	case cachedProposal != nil && t.uponProposalAndPolkaCurrent(cachedProposal):
		return t.doProposalAndPolkaCurrent(cachedProposal)

	// Line 44
	case t.uponPolkaNil():
		return t.doPolkaNil()

	// Line 47
	case t.uponPrecommitAny():
		return t.doPrecommitAny()

	// Line 49
	case roundCachedProposal != nil && t.uponCommitValue(roundCachedProposal):
		return t.doCommitValue(roundCachedProposal)

	// Line 55
	case recentlyReceivedRound != nil && t.uponSkipRound(*recentlyReceivedRound):
		return t.doSkipRound(*recentlyReceivedRound)

	default:
		return nil
	}
}
