package tendermint

func (t *Tendermint[V, H, A]) sendProposal(value *V) {
	proposalMessage := Proposal[V, H, A]{
		MessageHeader: MessageHeader[A]{
			Height: t.state.height,
			Round:  t.state.round,
			Sender: t.nodeAddr,
		},
		ValidRound: t.state.validRound,
		Value:      value,
	}

	t.messages.addProposal(proposalMessage)
	t.broadcasters.ProposalBroadcaster.Broadcast(proposalMessage)
}

func (t *Tendermint[V, H, A]) sendPrevote(id *H) {
	vote := Prevote[H, A]{
		MessageHeader: MessageHeader[A]{
			Height: t.state.height,
			Round:  t.state.round,
			Sender: t.nodeAddr,
		},
		ID: id,
	}

	t.messages.addPrevote(vote)
	t.broadcasters.PrevoteBroadcaster.Broadcast(vote)
	t.state.step = prevote
}

func (t *Tendermint[V, H, A]) sendPrecommit(id *H) {
	vote := Precommit[H, A]{
		MessageHeader: MessageHeader[A]{
			Height: t.state.height,
			Round:  t.state.round,
			Sender: t.nodeAddr,
		},
		ID: id,
	}

	t.messages.addPrecommit(vote)
	t.broadcasters.PrecommitBroadcaster.Broadcast(vote)
	t.state.step = precommit
}
