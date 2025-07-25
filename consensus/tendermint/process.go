package tendermint

import "github.com/NethermindEth/juno/consensus/types"

func (t *stateMachine[V, H, A]) ProcessStart(round types.Round) []types.Action[V, H, A] {
	return t.processLoop(t.startRound(round), nil)
}

func (t *stateMachine[V, H, A]) ProcessProposal(p *types.Proposal[V, H, A]) []types.Action[V, H, A] {
	return t.processMessage(p.MessageHeader, func() {
		if t.voteCounter.AddProposal(p) && !t.replayMode && p.Height == t.state.height {
			// Store proposal if its the first time we see it
			if err := t.db.SetWALEntry(p); err != nil {
				t.log.Fatalf("Failed to store prevote in WAL")
			}
		}
	})
}

func (t *stateMachine[V, H, A]) ProcessPrevote(p *types.Prevote[H, A]) []types.Action[V, H, A] {
	return t.processMessage(p.MessageHeader, func() {
		if t.voteCounter.AddPrevote(p) && !t.replayMode && p.Height == t.state.height {
			// Store prevote if its the first time we see it
			if err := t.db.SetWALEntry(p); err != nil {
				t.log.Fatalf("Failed to store prevote in WAL")
			}
		}
	})
}

func (t *stateMachine[V, H, A]) ProcessPrecommit(p *types.Precommit[H, A]) []types.Action[V, H, A] {
	return t.processMessage(p.MessageHeader, func() {
		if t.voteCounter.AddPrecommit(p) && !t.replayMode && p.Height == t.state.height {
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
	actions, shouldContinue := []types.Action[V, H, A]{}, true
	if action != nil {
		actions = append(actions, action)
	}

	for shouldContinue {
		action, shouldContinue = t.process(recentlyReceivedRound)
		if action != nil {
			actions = append(actions, action)
		}
	}

	return actions
}

func (t *stateMachine[V, H, A]) process(recentlyReceivedRound *types.Round) (action types.Action[V, H, A], shouldContinue bool) {
	cachedProposal := t.findProposal(t.state.round)

	var roundCachedProposal *CachedProposal[V, H, A]
	if recentlyReceivedRound != nil {
		roundCachedProposal = t.findProposal(*recentlyReceivedRound)
	}

	switch {
	// Line 22
	case cachedProposal != nil && t.uponFirstProposal(cachedProposal):
		return t.doFirstProposal(cachedProposal), true

	// Line 28
	case cachedProposal != nil && t.uponProposalAndPolkaPrevious(cachedProposal):
		return t.doProposalAndPolkaPrevious(cachedProposal), true

	// Line 34
	case t.uponPolkaAny():
		return t.doPolkaAny(), true

	// Line 36
	case cachedProposal != nil && t.uponProposalAndPolkaCurrent(cachedProposal):
		return t.doProposalAndPolkaCurrent(cachedProposal), true

	// Line 44
	case t.uponPolkaNil():
		return t.doPolkaNil(), true

	// Line 47
	case t.uponPrecommitAny():
		return t.doPrecommitAny(), true

	// Line 49
	case roundCachedProposal != nil && t.uponCommitValue(roundCachedProposal):
		return t.doCommitValue(roundCachedProposal), false // We should stop immediately after committing

	// Line 55
	case recentlyReceivedRound != nil && t.uponSkipRound(*recentlyReceivedRound):
		return t.doSkipRound(*recentlyReceivedRound), true

	default:
		return nil, false // We should stop if none of the above conditions are met
	}
}
