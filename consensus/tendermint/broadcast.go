package tendermint

import (
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/utils"
)

func (t *stateMachine[V]) sendProposal(value *V) types.Action[V] {
	proposalMessage := types.Proposal[V]{
		MessageHeader: types.MessageHeader{
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

	t.messages.AddProposal(proposalMessage)

	return utils.HeapPtr(types.BroadcastProposal[V](proposalMessage))
}

func (t *stateMachine[V]) setStepAndSendPrevote(id *types.Hash) types.Action[V] {
	vote := types.Prevote{
		MessageHeader: types.MessageHeader{
			Height: t.state.height,
			Round:  t.state.round,
			Sender: t.nodeAddr,
		},
		ID: id,
	}

	t.messages.AddPrevote(vote)
	t.state.step = types.StepPrevote

	return utils.HeapPtr(types.BroadcastPrevote(vote))
}

func (t *stateMachine[V]) setStepAndSendPrecommit(id *types.Hash) types.Action[V] {
	vote := types.Precommit{
		MessageHeader: types.MessageHeader{
			Height: t.state.height,
			Round:  t.state.round,
			Sender: t.nodeAddr,
		},
		ID: id,
	}

	t.messages.AddPrecommit(vote)
	t.state.step = types.StepPrecommit

	return utils.HeapPtr(types.BroadcastPrecommit(vote))
}
