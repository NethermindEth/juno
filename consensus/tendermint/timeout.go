package tendermint

import "github.com/NethermindEth/juno/consensus/types"

func (t *stateMachine[V, H, A]) onTimeoutPropose(h types.Height, r types.Round) types.Action[V, H, A] {
	if t.state.height == h && t.state.round == r && t.state.step == types.StepPropose {
		return t.setStepAndSendPrevote(nil)
	}
	return nil
}

func (t *stateMachine[V, H, A]) onTimeoutPrevote(h types.Height, r types.Round) types.Action[V, H, A] {
	if t.state.height == h && t.state.round == r && t.state.step == types.StepPrevote {
		return t.setStepAndSendPrecommit(nil)
	}
	return nil
}

func (t *stateMachine[V, H, A]) onTimeoutPrecommit(h types.Height, r types.Round) types.Action[V, H, A] {
	if t.state.height == h && t.state.round == r {
		return t.startRound(r + 1)
	}
	return nil
}
