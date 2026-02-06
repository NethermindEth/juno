package tendermint

import (
	"fmt"

	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/types/actions"
	"github.com/NethermindEth/juno/consensus/types/wal"
)

func (s *stateMachine[V, H, A]) ProcessStart(round types.Round) []actions.Action[V, H, A] {
	if s.isHeightStarted {
		return nil
	}
	s.isHeightStarted = true
	return s.processLoop(
		[]actions.Action[V, H, A]{
			&actions.WriteWAL[V, H, A]{Entry: (*wal.Start)(&s.state.height)},
			s.startRound(round),
		},
		nil,
	)
}

func (s *stateMachine[V, H, A]) ProcessProposal(p *types.Proposal[V, H, A]) []actions.Action[V, H, A] {
	if !s.voteCounter.AddProposal(p) || !s.isHeightStarted {
		return nil
	}
	return s.processMessage(p, (*wal.Proposal[V, H, A])(p))
}

func (s *stateMachine[V, H, A]) ProcessPrevote(p *types.Prevote[H, A]) []actions.Action[V, H, A] {
	if !s.voteCounter.AddPrevote(p) || !s.isHeightStarted {
		return nil
	}
	return s.processMessage(p, (*wal.Prevote[H, A])(p))
}

func (s *stateMachine[V, H, A]) ProcessPrecommit(p *types.Precommit[H, A]) []actions.Action[V, H, A] {
	if !s.voteCounter.AddPrecommit(p) || !s.isHeightStarted {
		return nil
	}
	return s.processMessage(p, (*wal.Precommit[H, A])(p))
}

func (s *stateMachine[V, H, A]) processMessage(
	msg types.Message[V, H, A],
	walEntry wal.Entry[V, H, A],
) []actions.Action[V, H, A] {
	actions := []actions.Action[V, H, A]{
		&actions.WriteWAL[V, H, A]{Entry: walEntry},
	}

	if msg.Header().Height != s.state.height {
		return actions
	}

	return s.processLoop(actions, &msg.Header().Round)
}

func (s *stateMachine[V, H, A]) ProcessTimeout(tm types.Timeout) []actions.Action[V, H, A] {
	switch tm.Step {
	case types.StepPropose:
		return s.processLoop(s.onTimeoutPropose(tm), nil)
	case types.StepPrevote:
		return s.processLoop(s.onTimeoutPrevote(tm), nil)
	case types.StepPrecommit:
		return s.processLoop(s.onTimeoutPrecommit(tm), nil)
	}

	return nil
}

func (s *stateMachine[V, H, A]) ProcessWAL(walEntry wal.Entry[V, H, A]) []actions.Action[V, H, A] {
	switch walEntry := walEntry.(type) {
	case *wal.Start:
		return s.ProcessStart(0)
	case *wal.Proposal[V, H, A]:
		return s.ProcessProposal((*types.Proposal[V, H, A])(walEntry))
	case *wal.Prevote[H, A]:
		return s.ProcessPrevote((*types.Prevote[H, A])(walEntry))
	case *wal.Precommit[H, A]:
		return s.ProcessPrecommit((*types.Precommit[H, A])(walEntry))
	case *wal.Timeout:
		return s.ProcessTimeout(types.Timeout(*walEntry))
	default:
		panic(fmt.Sprintf("unexpected WAL entry type: %T", walEntry))
	}
}

func (s *stateMachine[V, H, A]) processLoop(
	resultActions []actions.Action[V, H, A],
	recentlyReceivedRound *types.Round,
) []actions.Action[V, H, A] {
	var action actions.Action[V, H, A]
	shouldContinue := true
	for shouldContinue {
		action, shouldContinue = s.process(recentlyReceivedRound)
		if action != nil {
			resultActions = append(resultActions, action)
		}
	}

	return resultActions
}

func (s *stateMachine[V, H, A]) process(recentlyReceivedRound *types.Round) (action actions.Action[V, H, A], shouldContinue bool) {
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
