package tendermint

func (t *Tendermint[V, H, A]) sendProposal(value *V) {
	proposalMessage := Proposal[V, H, A]{
		H:          t.state.h,
		R:          t.state.r,
		ValidRound: t.state.validRound,
		Value:      value,
		Sender:     t.nodeAddr,
	}

	t.messages.addProposal(proposalMessage)
	t.broadcasters.ProposalBroadcaster.Broadcast(proposalMessage)
}

func (t *Tendermint[V, H, A]) sendPrevote(id *H) {
	vote := Prevote[H, A]{
		H:      t.state.h,
		R:      t.state.r,
		ID:     id,
		Sender: t.nodeAddr,
	}

	t.messages.addPrevote(vote)
	t.broadcasters.PrevoteBroadcaster.Broadcast(vote)
	t.state.s = prevote
}

func (t *Tendermint[V, H, A]) sendPrecommit(id *H) {
	vote := Precommit[H, A]{
		H:      t.state.h,
		R:      t.state.r,
		ID:     id,
		Sender: t.nodeAddr,
	}

	t.messages.addPrecommit(vote)
	t.broadcasters.PrecommitBroadcaster.Broadcast(vote)
	t.state.s = precommit
}
