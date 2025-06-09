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

func (t *stateMachine[V, H, A]) processLoop( //nolint:gocyclo
	initialAction types.Action[V, H, A],
	recentlyReceivedRound *types.Round,
) []types.Action[V, H, A] {
	actions := []types.Action[V, H, A]{}
	if initialAction != nil {
		actions = append(actions, initialAction)
	}

	// Process all rules until we can take no more actions
	for {
		cachedProposal := t.findProposal(t.state.round)

		var roundCachedProposal *CachedProposal[V, H, A]
		if recentlyReceivedRound != nil {
			roundCachedProposal = t.findProposal(*recentlyReceivedRound)
		}
		switch {
		// Line 22
		case cachedProposal != nil && t.uponFirstProposal(cachedProposal):
			if action := t.doFirstProposal(cachedProposal); action != nil {
				actions = append(actions, action)
			}

		// Line 28
		case cachedProposal != nil && t.uponProposalAndPolkaPrevious(cachedProposal):
			if action := t.doProposalAndPolkaPrevious(cachedProposal); action != nil {
				actions = append(actions, action)
			}

		// Line 34
		case t.uponPolkaAny():
			if action := t.doPolkaAny(); action != nil {
				actions = append(actions, action)
			}

		// Line 36
		case cachedProposal != nil && t.uponProposalAndPolkaCurrent(cachedProposal):
			if action := t.doProposalAndPolkaCurrent(cachedProposal); action != nil {
				actions = append(actions, action)
			}

		// Line 44
		case t.uponPolkaNil():
			if action := t.doPolkaNil(); action != nil {
				actions = append(actions, action)
			}

		// Line 47
		case t.uponPrecommitAny():
			if action := t.doPrecommitAny(); action != nil {
				actions = append(actions, action)
			}

		// Line 49
		case roundCachedProposal != nil && t.uponCommitValue(roundCachedProposal):
			if action := t.doCommitValue(); action != nil {
				actions = append(actions, action, (*types.Commit[V, H, A])(&roundCachedProposal.Proposal))
			}

		// Line 55
		case recentlyReceivedRound != nil && t.uponSkipRound(*recentlyReceivedRound):
			if action := t.doSkipRound(*recentlyReceivedRound); action != nil {
				actions = append(actions, action)
			}

		default:
			return actions
		}
	}
}
