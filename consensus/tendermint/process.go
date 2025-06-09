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
		case cachedProposal != nil && t.uponFirstProposal(cachedProposal):
			t.appendNonNil(&actions, t.doFirstProposal(cachedProposal))

		case cachedProposal != nil && t.uponProposalAndPolkaPrevious(cachedProposal):
			t.appendNonNil(&actions, t.doProposalAndPolkaPrevious(cachedProposal))

		case t.uponPolkaAny():
			t.appendNonNil(&actions, t.doPolkaAny())

		case cachedProposal != nil && t.uponProposalAndPolkaCurrent(cachedProposal):
			t.appendNonNil(&actions, t.doProposalAndPolkaCurrent(cachedProposal))

		case t.uponPolkaNil():
			t.appendNonNil(&actions, t.doPolkaNil())

		case t.uponPrecommitAny():
			t.appendNonNil(&actions, t.doPrecommitAny())

		case roundCachedProposal != nil && t.uponCommitValue(roundCachedProposal):
			t.appendNonNil(&actions, t.doCommitValue())
			actions = append(actions, (*types.Commit[V, H, A])(&roundCachedProposal.Proposal))

		case recentlyReceivedRound != nil && t.uponSkipRound(*recentlyReceivedRound):
			t.appendNonNil(&actions, t.doSkipRound(*recentlyReceivedRound))

		default:
			return actions
		}
	}
}

func (t *stateMachine[V, H, A]) appendNonNil(actions *[]types.Action[V, H, A], action types.Action[V, H, A]) {
	if action != nil {
		*actions = append(*actions, action)
	}
}
