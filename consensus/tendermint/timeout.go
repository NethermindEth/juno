package tendermint

func (t *Tendermint[_, H, A]) onTimeoutPropose(h height, r round) {
	if t.state.height == h && t.state.round == r && t.state.step == propose {
		t.sendPrevote(nil)
	}
}

func (t *Tendermint[_, H, A]) onTimeoutPrevote(h height, r round) {
	if t.state.height == h && t.state.round == r && t.state.step == prevote {
		t.sendPrecommit(nil)
	}
}

func (t *Tendermint[_, _, _]) onTimeoutPrecommit(h height, r round) {
	if t.state.height == h && t.state.round == r {
		t.startRound(r + 1)
	}
}
