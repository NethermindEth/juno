package tendermint

func (t *Tendermint[V, H, A]) sendProposal(value *V) {
	proposalMessage := Proposal[V, H, A]{
		Height:     t.state.height,
		Round:      t.state.round,
		ValidRound: t.state.validRound,
		Value:      value,
		Sender:     t.nodeAddr,
	}

	t.messages.addProposal(proposalMessage)
	t.broadcasters.ProposalBroadcaster.Broadcast(proposalMessage)
}

func (t *Tendermint[V, H, A]) sendPrevote(id *H) {
	vote := Prevote[H, A]{
		Height: t.state.height,
		Round:  t.state.round,
		ID:     id,
		Sender: t.nodeAddr,
	}

	t.messages.addPrevote(vote)
	t.broadcasters.PrevoteBroadcaster.Broadcast(vote)
	t.state.step = prevote
}

func (t *Tendermint[V, H, A]) sendPrecommit(id *H) {
	vote := Precommit[H, A]{
		Height: t.state.height,
		Round:  t.state.round,
		ID:     id,
		Sender: t.nodeAddr,
	}

	t.messages.addPrecommit(vote)
	t.broadcasters.PrecommitBroadcaster.Broadcast(vote)
	t.state.step = precommit
}
