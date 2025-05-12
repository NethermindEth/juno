package tendermint

import "github.com/NethermindEth/juno/consensus/types"

func (t *stateMachine[V, H, A]) ProcessStart(round types.Round) []types.Action[V, H, A] {
	return t.processLoop(t.startRound(round), nil)
}

func (t *stateMachine[V, H, A]) ProcessProposal(p types.Proposal[V, H, A]) []types.Action[V, H, A] {
	return t.processMessage(p.MessageHeader, func() {
		if t.messages.AddProposal(p) && !t.replayMode {
			// Store proposal if its the first time we see it
			if err := t.db.SetWALEntry(p); err != nil {
				t.log.Errorw("Failed to store prevote in WAL") // Todo: consider log level
			}
		}
	})
}

func (t *stateMachine[V, H, A]) ProcessPrevote(p types.Prevote[H, A]) []types.Action[V, H, A] {
	return t.processMessage(p.MessageHeader, func() {
		if t.messages.AddPrevote(p) && !t.replayMode {
			// Store prevote if its the first time we see it
			if err := t.db.SetWALEntry(p); err != nil {
				t.log.Errorw("Failed to store prevote in WAL") // Todo: consider log level
			}
		}
	})
}

func (t *stateMachine[V, H, A]) ProcessPrecommit(p types.Precommit[H, A]) []types.Action[V, H, A] {
	return t.processMessage(p.MessageHeader, func() {
		if t.messages.AddPrecommit(p) && !t.replayMode {
			// Store precommit if its the first time we see it
			if err := t.db.SetWALEntry(p); err != nil {
				t.log.Errorw("Failed to store prevote in WAL") // Todo: consider log level
			}
		}
	})
}

func (t *stateMachine[V, H, A]) processMessage(header types.MessageHeader[A], addMessage func()) []types.Action[V, H, A] {
	if !t.preprocessMessage(header, addMessage) {
		return nil
	}

	return t.processLoop(nil, &header.Round)
}

func (t *stateMachine[V, H, A]) ProcessTimeout(tm types.Timeout) []types.Action[V, H, A] {
	if err := t.db.SetWALEntry(tm); err != nil && !t.replayMode {
		t.log.Errorw("Failed to store timeout trigger in WAL") // Todo: consider log level
	}
	switch tm.Step {
	case types.StepPropose:
		return t.processLoop(t.onTimeoutPropose(tm.Height, tm.Round), nil)
	case types.StepPrevote:
		return t.processLoop(t.onTimeoutPrevote(tm.Height, tm.Round), nil)
	case types.StepPrecommit:
		return t.processLoop(t.onTimeoutPrecommit(tm.Height, tm.Round), nil)
	}

	return nil
}

func (t *stateMachine[V, H, A]) processLoop(action types.Action[V, H, A], recentlyReceivedRound *types.Round) []types.Action[V, H, A] {
	actions := []types.Action[V, H, A]{}
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

func (t *stateMachine[V, H, A]) process(recentlyReceivedRound *types.Round) types.Action[V, H, A] {
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
