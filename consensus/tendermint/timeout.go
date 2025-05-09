package tendermint

type Timeout struct {
	Step   Step
	Height Height
	Round  Round
	_      struct{} `cbor:",toarray"`
}

func (t Timeout) msgType() MessageType {
	return MessageTypeTimeout
}

func (t Timeout) height() Height {
	return t.Height
}

func (t *stateMachine[V, H, A]) onTimeoutPropose(h Height, r Round) Action[V, H, A] {
	if t.state.height == h && t.state.round == r && t.state.step == StepPropose {
		return t.setStepAndSendPrevote(nil)
	}
	return nil
}

func (t *stateMachine[V, H, A]) onTimeoutPrevote(h Height, r Round) Action[V, H, A] {
	if t.state.height == h && t.state.round == r && t.state.step == StepPrevote {
		return t.setStepAndSendPrecommit(nil)
	}
	return nil
}

func (t *stateMachine[V, H, A]) onTimeoutPrecommit(h Height, r Round) Action[V, H, A] {
	if t.state.height == h && t.state.round == r {
		return t.startRound(r + 1)
	}
	return nil
}
