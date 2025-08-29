package tendermint

import (
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/types/wal"
)

func (s *stateMachine[V, H, A]) ProcessStart(round types.Round) []types.Action[V, H, A] {
	return s.processLoop(s.startRound(round), nil)
}

func (s *stateMachine[V, H, A]) ProcessProposal(p *types.Proposal[V, H, A]) []types.Action[V, H, A] {
	return s.processMessage(p.MessageHeader, func() {
		if s.voteCounter.AddProposal(p) && !s.replayMode && p.Height == s.state.height {
			// Store proposal if its the first time we see it
			if err := s.db.SetWALEntry((*wal.WALProposal[V, H, A])(p)); err != nil {
				s.log.Fatalf("Failed to store prevote in WAL")
			}
		}
	})
}

func (s *stateMachine[V, H, A]) ProcessPrevote(p *types.Prevote[H, A]) []types.Action[V, H, A] {
	return s.processMessage(p.MessageHeader, func() {
		if s.voteCounter.AddPrevote(p) && !s.replayMode && p.Height == s.state.height {
			// Store prevote if its the first time we see it
			if err := s.db.SetWALEntry((*wal.WALPrevote[H, A])(p)); err != nil {
				s.log.Fatalf("Failed to store prevote in WAL")
			}
		}
	})
}

func (s *stateMachine[V, H, A]) ProcessPrecommit(p *types.Precommit[H, A]) []types.Action[V, H, A] {
	return s.processMessage(p.MessageHeader, func() {
		if s.voteCounter.AddPrecommit(p) && !s.replayMode && p.Height == s.state.height {
			// Store precommit if its the first time we see it
			if err := s.db.SetWALEntry((*wal.WALPrecommit[H, A])(p)); err != nil {
				s.log.Fatalf("Failed to store prevote in WAL")
			}
		}
	})
}

func (s *stateMachine[V, H, A]) processMessage(header types.MessageHeader[A], addMessage func()) []types.Action[V, H, A] {
	if !s.preprocessMessage(header, addMessage) {
		return nil
	}

	return s.processLoop(nil, &header.Round)
}

func (s *stateMachine[V, H, A]) ProcessTimeout(tm types.Timeout) []types.Action[V, H, A] {
	if !s.replayMode && tm.Height == s.state.height {
		if err := s.db.SetWALEntry((*wal.WALTimeout)(&tm)); err != nil {
			s.log.Fatalf("Failed to store timeout trigger in WAL")
		}
	}
	switch tm.Step {
	case types.StepPropose:
		return s.processLoop(s.onTimeoutPropose(tm.Height, tm.Round), nil)
	case types.StepPrevote:
		return s.processLoop(s.onTimeoutPrevote(tm.Height, tm.Round), nil)
	case types.StepPrecommit:
		return s.processLoop(s.onTimeoutPrecommit(tm.Height, tm.Round), nil)
	}

	return nil
}

func (s *stateMachine[V, H, A]) processLoop(action types.Action[V, H, A], recentlyReceivedRound *types.Round) []types.Action[V, H, A] {
	actions, shouldContinue := []types.Action[V, H, A]{}, true
	if action != nil {
		actions = append(actions, action)
	}

	for shouldContinue {
		action, shouldContinue = s.process(recentlyReceivedRound)
		if action != nil {
			actions = append(actions, action)
		}
	}

	return actions
}

func (s *stateMachine[V, H, A]) process(recentlyReceivedRound *types.Round) (action types.Action[V, H, A], shouldContinue bool) {
	cachedProposal := s.findProposal(s.state.round)

	roundCachedProposal := cachedProposal
	if recentlyReceivedRound != nil {
		roundCachedProposal = s.findProposal(*recentlyReceivedRound)
	}

	switch {
	// Line 22
	case cachedProposal != nil && s.uponFirstProposal(cachedProposal):
		return s.doFirstProposal(cachedProposal), true

	// Line 28
	case cachedProposal != nil && s.uponProposalAndPolkaPrevious(cachedProposal):
		return s.doProposalAndPolkaPrevious(cachedProposal), true

	// Line 34
	case s.uponPolkaAny():
		return s.doPolkaAny(), true

	// Line 36
	case cachedProposal != nil && s.uponProposalAndPolkaCurrent(cachedProposal):
		return s.doProposalAndPolkaCurrent(cachedProposal), true

	// Line 44
	case s.uponPolkaNil():
		return s.doPolkaNil(), true

	// Line 47
	case s.uponPrecommitAny():
		return s.doPrecommitAny(), true

	// Line 49
	case roundCachedProposal != nil && s.uponCommitValue(roundCachedProposal):
		return s.doCommitValue(roundCachedProposal), false // We should stop immediately after committing

	// Line 55
	case recentlyReceivedRound != nil && s.uponSkipRound(*recentlyReceivedRound):
		return s.doSkipRound(*recentlyReceivedRound), true

	default:
		return nil, false // We should stop if none of the above conditions are met
	}
}
