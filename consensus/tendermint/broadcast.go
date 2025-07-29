package tendermint

import (
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/utils"
)

func (s *stateMachine[V, H, A]) sendProposal(value *V) types.Action[V, H, A] {
	proposalMessage := types.Proposal[V, H, A]{
		MessageHeader: types.MessageHeader[A]{
			Height: s.state.height,
			Round:  s.state.round,
			Sender: s.nodeAddr,
		},
		ValidRound: s.state.validRound,
		Value:      value,
	}

	if err := s.db.SetWALEntry(proposalMessage); err != nil && !s.replayMode {
		s.log.Fatalf("Failed to store propsal in WAL")
	}

	s.voteCounter.AddProposal(&proposalMessage)

	return utils.HeapPtr(types.BroadcastProposal[V, H, A](proposalMessage))
}

func (s *stateMachine[V, H, A]) setStepAndSendPrevote(id *H) types.Action[V, H, A] {
	vote := types.Prevote[H, A]{
		MessageHeader: types.MessageHeader[A]{
			Height: s.state.height,
			Round:  s.state.round,
			Sender: s.nodeAddr,
		},
		ID: id,
	}

	s.voteCounter.AddPrevote(&vote)
	s.state.step = types.StepPrevote

	return utils.HeapPtr(types.BroadcastPrevote[H, A](vote))
}

func (s *stateMachine[V, H, A]) setStepAndSendPrecommit(id *H) types.Action[V, H, A] {
	vote := types.Precommit[H, A]{
		MessageHeader: types.MessageHeader[A]{
			Height: s.state.height,
			Round:  s.state.round,
			Sender: s.nodeAddr,
		},
		ID: id,
	}

	s.voteCounter.AddPrecommit(&vote)
	s.state.step = types.StepPrecommit

	return utils.HeapPtr(types.BroadcastPrecommit[H, A](vote))
}
