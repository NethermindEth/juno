package tendermint

import (
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/types/actions"
	"github.com/NethermindEth/juno/consensus/types/wal"
)

func (s *stateMachine[V, H, A]) onTimeoutPropose(
	timeout types.Timeout,
) []actions.Action[V, H, A] {
	isSameHeightAndRound := s.state.height == timeout.Height && s.state.round == timeout.Round
	isProposeStep := s.state.step == types.StepPropose

	if isSameHeightAndRound && isProposeStep {
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
	isSameHeightAndRound := s.state.height == timeout.Height && s.state.round == timeout.Round
	isPrevoteStep := s.state.step == types.StepPrevote

	if isSameHeightAndRound && isPrevoteStep {
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
	isSameHeightAndRound := s.state.height == timeout.Height && s.state.round == timeout.Round

	if isSameHeightAndRound {
		return []actions.Action[V, H, A]{
			&actions.WriteWAL[V, H, A]{Entry: (*wal.Timeout)(&timeout)},
			s.startRound(timeout.Round + 1),
		}
	}
	return nil
}
