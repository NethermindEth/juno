package tendermint

import "github.com/NethermindEth/juno/utils"

func (t *Tendermint[V, H, A]) sendProposal(value *V) Action[V, H, A] {
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

	return utils.HeapPtr(BroadcastProposal[V, H, A](proposalMessage))
}

func (t *Tendermint[V, H, A]) sendPrevote(id *H) Action[V, H, A] {
	vote := Prevote[H, A]{
		MessageHeader: MessageHeader[A]{
			Height: t.state.height,
			Round:  t.state.round,
			Sender: t.nodeAddr,
		},
		ID: id,
	}

	t.messages.addPrevote(vote)
	t.state.step = prevote

	return utils.HeapPtr(BroadcastPrevote[V, H, A](vote))
}

func (t *Tendermint[V, H, A]) sendPrecommit(id *H) Action[V, H, A] {
	vote := Precommit[H, A]{
		MessageHeader: MessageHeader[A]{
			Height: t.state.height,
			Round:  t.state.round,
			Sender: t.nodeAddr,
		},
		ID: id,
	}

	t.messages.addPrecommit(vote)
	t.state.step = precommit

	return utils.HeapPtr(BroadcastPrecommit[V, H, A](vote))
}
