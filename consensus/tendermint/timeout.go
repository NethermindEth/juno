package tendermint

import (
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/types/actions"
)

func (s *stateMachine[V, H, A]) onTimeoutPropose(h types.Height, r types.Round) actions.Action[V, H, A] {
	if s.state.height == h && s.state.round == r && s.state.step == types.StepPropose {
		return s.setStepAndSendPrevote(nil)
	}
	return nil
}

func (s *stateMachine[V, H, A]) onTimeoutPrevote(h types.Height, r types.Round) actions.Action[V, H, A] {
	if s.state.height == h && s.state.round == r && s.state.step == types.StepPrevote {
		return s.setStepAndSendPrecommit(nil)
	}
	return nil
}

func (s *stateMachine[V, H, A]) onTimeoutPrecommit(h types.Height, r types.Round) actions.Action[V, H, A] {
	if s.state.height == h && s.state.round == r {
		return s.startRound(r + 1)
	}
	return nil
}
