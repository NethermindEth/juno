package tendermint

import "github.com/NethermindEth/juno/utils"

func (t *stateMachine[V, H, A]) sendProposal(value *V) Action[V, H, A] {
	proposalMessage := Proposal[V, H, A]{
		MessageHeader: MessageHeader[A]{
			Height: t.state.height,
			Round:  t.state.round,
			Sender: t.nodeAddr,
		},
		ValidRound: t.state.validRound,
		Value:      value,
	}

	if err := t.db.SetWALEntry(proposalMessage); err != nil && !t.replayMode {
		t.log.Errorw("Failed to store propsal in WAL") // Todo: consider log level
	}

	t.messages.addProposal(proposalMessage)

	return utils.HeapPtr(BroadcastProposal[V, H, A](proposalMessage))
}

func (t *stateMachine[V, H, A]) setStepAndSendPrevote(id *H) Action[V, H, A] {
	vote := Prevote[H, A]{
		MessageHeader: MessageHeader[A]{
			Height: t.state.height,
			Round:  t.state.round,
			Sender: t.nodeAddr,
		},
		ID: id,
	}

	t.messages.addPrevote(vote)
	t.state.step = StepPrevote

	return utils.HeapPtr(BroadcastPrevote[H, A](vote))
}

func (t *stateMachine[V, H, A]) setStepAndSendPrecommit(id *H) Action[V, H, A] {
	vote := Precommit[H, A]{
		MessageHeader: MessageHeader[A]{
			Height: t.state.height,
			Round:  t.state.round,
			Sender: t.nodeAddr,
		},
		ID: id,
	}

	t.messages.addPrecommit(vote)
	t.state.step = StepPrecommit

	return utils.HeapPtr(BroadcastPrecommit[H, A](vote))
}
