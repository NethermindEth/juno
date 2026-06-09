package tendermint

import (
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/types/actions"
	"github.com/NethermindEth/juno/consensus/types/wal"
)

// todo(rdr): Should we ignore calling this function outside the right step?

func (s *stateMachine[V, H, A]) onTimeoutPropose(
	timeout types.Timeout,
) []actions.Action[V, H, A] {
	isProposeStep := s.state.step == types.StepPropose

	if s.isSameHeightAndRound(&timeout) && isProposeStep {
		return []actions.Action[V, H, A]{
			&actions.WriteWAL[V, H, A]{Entry: (*wal.Timeout)(&timeout)},
			s.setStepAndSendPrevote(nil),
		}
	}
	return nil
}

func (s *stateMachine[V, H, A]) onTimeoutPrevote(
	timeout types.Timeout,
) []actions.Action[V, H, A] {
	isPrevoteStep := s.state.step == types.StepPrevote

	if s.isSameHeightAndRound(&timeout) && isPrevoteStep {
		return []actions.Action[V, H, A]{
			&actions.WriteWAL[V, H, A]{Entry: (*wal.Timeout)(&timeout)},
			s.setStepAndSendPrecommit(nil),
		}
	}
	return nil
}

func (s *stateMachine[V, H, A]) onTimeoutPrecommit(
	timeout types.Timeout,
) []actions.Action[V, H, A] {
	if s.isSameHeightAndRound(&timeout) {
		return []actions.Action[V, H, A]{
			&actions.WriteWAL[V, H, A]{Entry: (*wal.Timeout)(&timeout)},
			s.startRound(timeout.Round + 1),
		}
	}
	return nil
}

func (s *stateMachine[V, H, A]) isSameHeightAndRound(timeout *types.Timeout) bool {
	return s.state.height == timeout.Height && s.state.round == timeout.Round
}
