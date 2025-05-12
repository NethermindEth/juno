package tendermint

import (
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/utils"
)

type BroadcastProposal[V types.Hashable[H], H types.Hash, A types.Addr] types.Proposal[V, H, A]

type BroadcastPrevote[H types.Hash, A types.Addr] types.Prevote[H, A]

type BroadcastPrecommit[H types.Hash, A types.Addr] types.Precommit[H, A]

type ScheduleTimeout types.Timeout

func (a *BroadcastProposal[V, H, A]) IsTendermintAction() {}

func (a *BroadcastPrevote[H, A]) IsTendermintAction() {}

func (a *BroadcastPrecommit[H, A]) IsTendermintAction() {}

func (a *ScheduleTimeout) IsTendermintAction() {}

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
		t.log.Errorw("Failed to store propsal in WAL") // Todo: consider log level
	}

	t.messages.AddProposal(proposalMessage)

	return utils.HeapPtr(BroadcastProposal[V, H, A](proposalMessage))
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

	t.messages.AddPrevote(vote)
	t.state.step = types.StepPrevote

	return utils.HeapPtr(BroadcastPrevote[H, A](vote))
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

	t.messages.AddPrecommit(vote)
	t.state.step = types.StepPrecommit

	return utils.HeapPtr(BroadcastPrecommit[H, A](vote))
}
