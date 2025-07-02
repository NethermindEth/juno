package tendermint

import "github.com/NethermindEth/juno/consensus/types"

func (t *stateMachine[V, H, A]) ProcessStart(round types.Round) []types.Action[V, H, A] {
	return t.processLoop(t.startRound(round), nil)
}

func (t *stateMachine[V, H, A]) ProcessProposal(p types.Proposal[V, H, A]) []types.Action[V, H, A] {
	return t.processMessage(p.MessageHeader, func() {
		if t.messages.AddProposal(p) && !t.replayMode && p.Height == t.state.height {
			// Store proposal if its the first time we see it
			if err := t.db.SetWALEntry(p); err != nil {
				t.log.Fatalf("Failed to store prevote in WAL")
			}
		}
	})
}

func (t *stateMachine[V, H, A]) ProcessPrevote(p types.Prevote[H, A]) []types.Action[V, H, A] {
	return t.processMessage(p.MessageHeader, func() {
		if t.messages.AddPrevote(p) && !t.replayMode && p.Height == t.state.height {
			// Store prevote if its the first time we see it
			if err := t.db.SetWALEntry(p); err != nil {
				t.log.Fatalf("Failed to store prevote in WAL")
			}
		}
	})
}

func (t *stateMachine[V, H, A]) ProcessSyncedBlock() []types.Action[V, H, A] {
	return []types.Action[V, H, A]{t.doCommitValue()}
}

func (t *stateMachine[V, H, A]) ProcessPrecommit(p types.Precommit[H, A]) []types.Action[V, H, A] {
	return t.processMessage(p.MessageHeader, func() {
		if t.messages.AddPrecommit(p) && !t.replayMode && p.Height == t.state.height {
			// Store precommit if its the first time we see it
			if err := t.db.SetWALEntry(p); err != nil {
				t.log.Fatalf("Failed to store prevote in WAL")
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
	if !t.replayMode && tm.Height == t.state.height {
		if err := t.db.SetWALEntry(tm); err != nil {
			t.log.Fatalf("Failed to store timeout trigger in WAL")
		}
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
	actions := appendAction[V, H, A](nil, action)
	// Always try processing at least once
	shouldContinue := true

	for shouldContinue {
		actions, shouldContinue = t.process(actions, recentlyReceivedRound)
	}

	return actions
}

func (t *stateMachine[V, H, A]) process(
	existingActions []types.Action[V, H, A],
	recentlyReceivedRound *types.Round,
) (newActions []types.Action[V, H, A], shouldContinue bool) {
	cachedProposal := t.findProposal(t.state.round)

	var roundCachedProposal *CachedProposal[V, H, A]
	if recentlyReceivedRound != nil {
		roundCachedProposal = t.findProposal(*recentlyReceivedRound)
	}

	switch {
	// Line 22
	case cachedProposal != nil && t.uponFirstProposal(cachedProposal):
		return appendAction(existingActions, t.doFirstProposal(cachedProposal)), true

	// Line 28
	case cachedProposal != nil && t.uponProposalAndPolkaPrevious(cachedProposal):
		return appendAction(existingActions, t.doProposalAndPolkaPrevious(cachedProposal)), true

	// Line 34
	case t.uponPolkaAny():
		return appendAction(existingActions, t.doPolkaAny()), true

	// Line 36
	case cachedProposal != nil && t.uponProposalAndPolkaCurrent(cachedProposal):
		return appendAction(existingActions, t.doProposalAndPolkaCurrent(cachedProposal)), true

	// Line 44
	case t.uponPolkaNil():
		return appendAction(existingActions, t.doPolkaNil()), true

	// Line 47
	case t.uponPrecommitAny():
		return appendAction(existingActions, t.doPrecommitAny()), true

	// Line 49
	case roundCachedProposal != nil && t.uponCommitValue(roundCachedProposal):
		return appendAction(append(existingActions, (*types.Commit[V, H, A])(&roundCachedProposal.Proposal)), t.doCommitValue()), true

	// Line 55
	case recentlyReceivedRound != nil && t.uponSkipRound(*recentlyReceivedRound):
		return appendAction(existingActions, t.doSkipRound(*recentlyReceivedRound)), true

	default:
		return existingActions, false
	}
}

func appendAction[V types.Hashable[H], H types.Hash, A types.Addr](
	existingActions []types.Action[V, H, A],
	action types.Action[V, H, A],
) []types.Action[V, H, A] {
	if action != nil {
		return append(existingActions, action)
	}
	return existingActions
}
