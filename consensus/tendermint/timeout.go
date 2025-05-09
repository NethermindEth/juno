package tendermint

type timeout struct {
	Step   step
	Height height
	Round  round
	_      struct{} `cbor:",toarray"`
}

func (t timeout) msgType() MessageType {
	return MessageTypeTimeout
}

func (t timeout) height() height {
	return t.Height
}

func (t *Tendermint[V, H, A]) onTimeoutPropose(h height, r round) Action[V, H, A] {
	if t.state.height == h && t.state.round == r && t.state.step == propose {
		return t.setStepAndSendPrevote(nil)
	}
	return nil
}

func (t *Tendermint[V, H, A]) onTimeoutPrevote(h height, r round) Action[V, H, A] {
	if t.state.height == h && t.state.round == r && t.state.step == prevote {
		return t.setStepAndSendPrecommit(nil)
	}
	return nil
}

func (t *Tendermint[V, H, A]) onTimeoutPrecommit(h height, r round) Action[V, H, A] {
	if t.state.height == h && t.state.round == r {
		return t.startRound(r + 1)
	}
	return nil
}
