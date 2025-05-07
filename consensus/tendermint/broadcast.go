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

	// Store the proposal in the WAL
	if err := t.db.SetWALEntry(proposalMessage, t.state.height); err != nil {
		t.log.Errorw("Failed to store propsal in WAL") // Todo: consider log level
	}

	t.messages.addProposal(proposalMessage)

	return utils.HeapPtr(BroadcastProposal[V, H, A](proposalMessage))
}

func (t *Tendermint[V, H, A]) setStepAndSendPrevote(id *H) Action[V, H, A] {
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

	return utils.HeapPtr(BroadcastPrevote[H, A](vote))
}

func (t *Tendermint[V, H, A]) setStepAndSendPrecommit(id *H) Action[V, H, A] {
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

	return utils.HeapPtr(BroadcastPrecommit[H, A](vote))
}
