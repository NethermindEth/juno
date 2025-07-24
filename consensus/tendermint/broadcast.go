package tendermint

import (
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/utils"
)

func (t *stateMachine[V, H, A]) sendProposal(value *V) types.Action[V, H, A] {
	proposalMessage := types.Proposal[V, H, A]{
		MessageHeader: types.MessageHeader[A]{
			Height: t.state.height,
			Round:  t.state.round,
			Sender: t.nodeAddr,
		},
		ValidRound: t.state.validRound,
		Value:      value,
	}

	if err := t.db.SetWALEntry(proposalMessage); err != nil && !t.replayMode {
		t.log.Fatalf("Failed to store propsal in WAL")
	}

	t.voteCounter.AddProposal(&proposalMessage)

	return utils.HeapPtr(types.BroadcastProposal[V, H, A](proposalMessage))
}

func (t *stateMachine[V, H, A]) setStepAndSendPrevote(id *H) types.Action[V, H, A] {
	vote := types.Prevote[H, A]{
		MessageHeader: types.MessageHeader[A]{
			Height: t.state.height,
			Round:  t.state.round,
			Sender: t.nodeAddr,
		},
		ID: id,
	}

	t.voteCounter.AddPrevote(&vote)
	t.state.step = types.StepPrevote

	return utils.HeapPtr(types.BroadcastPrevote[H, A](vote))
}

func (t *stateMachine[V, H, A]) setStepAndSendPrecommit(id *H) types.Action[V, H, A] {
	vote := types.Precommit[H, A]{
		MessageHeader: types.MessageHeader[A]{
			Height: t.state.height,
			Round:  t.state.round,
			Sender: t.nodeAddr,
		},
		ID: id,
	}

	t.voteCounter.AddPrecommit(&vote)
	t.state.step = types.StepPrecommit

	return utils.HeapPtr(types.BroadcastPrecommit[H, A](vote))
}
